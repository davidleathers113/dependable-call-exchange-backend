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
)

// DNCProviderRepository implements the dnc.DNCProviderRepository interface using PostgreSQL
// Performance targets:
// - Save: < 20ms for single provider
// - FindByType: < 10ms with proper indexing
// - List: < 50ms for full provider list
type DNCProviderRepository struct {
	db *pgxpool.Pool
}

// NewDNCProviderRepository creates a new PostgreSQL DNC provider repository
func NewDNCProviderRepository(db *pgxpool.Pool) *DNCProviderRepository {
	return &DNCProviderRepository{db: db}
}

// Save creates or updates a DNC provider
func (r *DNCProviderRepository) Save(ctx context.Context, provider *dnc.DNCProvider) error {
	query := `
		INSERT INTO dnc_providers (
			id, name, type, base_url, auth_type, api_key, update_frequency,
			last_sync_at, next_sync_at, status, enabled, priority, retry_attempts,
			timeout_seconds, rate_limit_per_min, last_sync_duration, last_sync_records,
			last_error, error_count, success_count, config, created_at, updated_at,
			created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21, $22, $23, $24, $25
		)
		ON CONFLICT (name) 
		DO UPDATE SET
			type = EXCLUDED.type,
			base_url = EXCLUDED.base_url,
			auth_type = EXCLUDED.auth_type,
			api_key = EXCLUDED.api_key,
			update_frequency = EXCLUDED.update_frequency,
			status = EXCLUDED.status,
			enabled = EXCLUDED.enabled,
			priority = EXCLUDED.priority,
			retry_attempts = EXCLUDED.retry_attempts,
			timeout_seconds = EXCLUDED.timeout_seconds,
			rate_limit_per_min = EXCLUDED.rate_limit_per_min,
			config = EXCLUDED.config,
			updated_at = EXCLUDED.updated_at,
			updated_by = EXCLUDED.updated_by
	`

	configJSON, err := json.Marshal(provider.Config)
	if err != nil {
		return errors.NewInternalError("failed to marshal config").WithCause(err)
	}

	_, err = r.db.Exec(ctx, query,
		provider.ID,
		provider.Name,
		string(provider.Type),
		provider.BaseURL,
		string(provider.AuthType),
		provider.APIKey,
		provider.UpdateFrequency,
		provider.LastSyncAt,
		provider.NextSyncAt,
		string(provider.Status),
		provider.Enabled,
		provider.Priority,
		provider.RetryAttempts,
		provider.TimeoutSeconds,
		provider.RateLimitPerMin,
		provider.LastSyncDuration,
		provider.LastSyncRecords,
		provider.LastError,
		provider.ErrorCount,
		provider.SuccessCount,
		configJSON,
		provider.CreatedAt,
		provider.UpdatedAt,
		provider.CreatedBy,
		provider.UpdatedBy,
	)

	if err != nil {
		return errors.NewInternalError("failed to save DNC provider").WithCause(err)
	}

	return nil
}

// SaveWithTx saves a DNC provider within an existing transaction
func (r *DNCProviderRepository) SaveWithTx(ctx context.Context, tx dnc.Transaction, provider *dnc.DNCProvider) error {
	pgxTx := tx.(*PgxTransaction)
	
	query := `
		INSERT INTO dnc_providers (
			id, name, type, base_url, auth_type, api_key, update_frequency,
			last_sync_at, next_sync_at, status, enabled, priority, retry_attempts,
			timeout_seconds, rate_limit_per_min, last_sync_duration, last_sync_records,
			last_error, error_count, success_count, config, created_at, updated_at,
			created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21, $22, $23, $24, $25
		)
		ON CONFLICT (name) 
		DO UPDATE SET
			type = EXCLUDED.type,
			base_url = EXCLUDED.base_url,
			auth_type = EXCLUDED.auth_type,
			api_key = EXCLUDED.api_key,
			update_frequency = EXCLUDED.update_frequency,
			status = EXCLUDED.status,
			enabled = EXCLUDED.enabled,
			priority = EXCLUDED.priority,
			retry_attempts = EXCLUDED.retry_attempts,
			timeout_seconds = EXCLUDED.timeout_seconds,
			rate_limit_per_min = EXCLUDED.rate_limit_per_min,
			config = EXCLUDED.config,
			updated_at = EXCLUDED.updated_at,
			updated_by = EXCLUDED.updated_by
	`

	configJSON, err := json.Marshal(provider.Config)
	if err != nil {
		return errors.NewInternalError("failed to marshal config").WithCause(err)
	}

	_, err = pgxTx.tx.Exec(ctx, query,
		provider.ID,
		provider.Name,
		string(provider.Type),
		provider.BaseURL,
		string(provider.AuthType),
		provider.APIKey,
		provider.UpdateFrequency,
		provider.LastSyncAt,
		provider.NextSyncAt,
		string(provider.Status),
		provider.Enabled,
		provider.Priority,
		provider.RetryAttempts,
		provider.TimeoutSeconds,
		provider.RateLimitPerMin,
		provider.LastSyncDuration,
		provider.LastSyncRecords,
		provider.LastError,
		provider.ErrorCount,
		provider.SuccessCount,
		configJSON,
		provider.CreatedAt,
		provider.UpdatedAt,
		provider.CreatedBy,
		provider.UpdatedBy,
	)

	if err != nil {
		return errors.NewInternalError("failed to save DNC provider in transaction").WithCause(err)
	}

	return nil
}

// GetByID retrieves a DNC provider by its unique identifier
func (r *DNCProviderRepository) GetByID(ctx context.Context, id uuid.UUID) (*dnc.DNCProvider, error) {
	query := `
		SELECT id, name, type, base_url, auth_type, api_key, update_frequency,
			   last_sync_at, next_sync_at, status, enabled, priority, retry_attempts,
			   timeout_seconds, rate_limit_per_min, last_sync_duration, last_sync_records,
			   last_error, error_count, success_count, config, created_at, updated_at,
			   created_by, updated_by
		FROM dnc_providers
		WHERE id = $1 AND deleted_at IS NULL
	`

	row := r.db.QueryRow(ctx, query, id)
	return r.scanDNCProvider(row)
}

// Update modifies an existing DNC provider
func (r *DNCProviderRepository) Update(ctx context.Context, provider *dnc.DNCProvider) error {
	query := `
		UPDATE dnc_providers 
		SET name = $2, type = $3, base_url = $4, auth_type = $5, api_key = $6,
		    update_frequency = $7, status = $8, enabled = $9, priority = $10,
		    retry_attempts = $11, timeout_seconds = $12, rate_limit_per_min = $13,
		    config = $14, updated_at = $15, updated_by = $16
		WHERE id = $1 AND deleted_at IS NULL
	`

	configJSON, err := json.Marshal(provider.Config)
	if err != nil {
		return errors.NewInternalError("failed to marshal config").WithCause(err)
	}

	result, err := r.db.Exec(ctx, query,
		provider.ID,
		provider.Name,
		string(provider.Type),
		provider.BaseURL,
		string(provider.AuthType),
		provider.APIKey,
		provider.UpdateFrequency,
		string(provider.Status),
		provider.Enabled,
		provider.Priority,
		provider.RetryAttempts,
		provider.TimeoutSeconds,
		provider.RateLimitPerMin,
		configJSON,
		provider.UpdatedAt,
		provider.UpdatedBy,
	)

	if err != nil {
		return errors.NewInternalError("failed to update DNC provider").WithCause(err)
	}

	if result.RowsAffected() == 0 {
		return errors.NewNotFoundError("DNC provider not found")
	}

	return nil
}

// UpdateWithTx updates a DNC provider within an existing transaction
func (r *DNCProviderRepository) UpdateWithTx(ctx context.Context, tx dnc.Transaction, provider *dnc.DNCProvider) error {
	pgxTx := tx.(*PgxTransaction)
	
	query := `
		UPDATE dnc_providers 
		SET name = $2, type = $3, base_url = $4, auth_type = $5, api_key = $6,
		    update_frequency = $7, status = $8, enabled = $9, priority = $10,
		    retry_attempts = $11, timeout_seconds = $12, rate_limit_per_min = $13,
		    config = $14, updated_at = $15, updated_by = $16
		WHERE id = $1 AND deleted_at IS NULL
	`

	configJSON, err := json.Marshal(provider.Config)
	if err != nil {
		return errors.NewInternalError("failed to marshal config").WithCause(err)
	}

	result, err := pgxTx.tx.Exec(ctx, query,
		provider.ID,
		provider.Name,
		string(provider.Type),
		provider.BaseURL,
		string(provider.AuthType),
		provider.APIKey,
		provider.UpdateFrequency,
		string(provider.Status),
		provider.Enabled,
		provider.Priority,
		provider.RetryAttempts,
		provider.TimeoutSeconds,
		provider.RateLimitPerMin,
		configJSON,
		provider.UpdatedAt,
		provider.UpdatedBy,
	)

	if err != nil {
		return errors.NewInternalError("failed to update DNC provider in transaction").WithCause(err)
	}

	if result.RowsAffected() == 0 {
		return errors.NewNotFoundError("DNC provider not found")
	}

	return nil
}

// Delete removes a DNC provider (soft delete to preserve audit trail)
func (r *DNCProviderRepository) Delete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error {
	query := `
		UPDATE dnc_providers 
		SET deleted_at = NOW(), deleted_by = $2, updated_at = NOW(), updated_by = $2
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.Exec(ctx, query, id, deletedBy)
	if err != nil {
		return errors.NewInternalError("failed to delete DNC provider").WithCause(err)
	}

	if result.RowsAffected() == 0 {
		return errors.NewNotFoundError("DNC provider not found")
	}

	return nil
}

// FindByType retrieves providers by provider type
func (r *DNCProviderRepository) FindByType(ctx context.Context, providerType dnc.ProviderType) ([]*dnc.DNCProvider, error) {
	query := `
		SELECT id, name, type, base_url, auth_type, api_key, update_frequency,
			   last_sync_at, next_sync_at, status, enabled, priority, retry_attempts,
			   timeout_seconds, rate_limit_per_min, last_sync_duration, last_sync_records,
			   last_error, error_count, success_count, config, created_at, updated_at,
			   created_by, updated_by
		FROM dnc_providers
		WHERE type = $1 AND deleted_at IS NULL
		ORDER BY priority ASC, name ASC
	`

	rows, err := r.db.Query(ctx, query, string(providerType))
	if err != nil {
		return nil, errors.NewInternalError("failed to query DNC providers by type").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCProviders(rows)
}

// FindByName retrieves a provider by its unique name
func (r *DNCProviderRepository) FindByName(ctx context.Context, name string) (*dnc.DNCProvider, error) {
	query := `
		SELECT id, name, type, base_url, auth_type, api_key, update_frequency,
			   last_sync_at, next_sync_at, status, enabled, priority, retry_attempts,
			   timeout_seconds, rate_limit_per_min, last_sync_duration, last_sync_records,
			   last_error, error_count, success_count, config, created_at, updated_at,
			   created_by, updated_by
		FROM dnc_providers
		WHERE name = $1 AND deleted_at IS NULL
	`

	row := r.db.QueryRow(ctx, query, name)
	return r.scanDNCProvider(row)
}

// List retrieves all providers with optional filtering
func (r *DNCProviderRepository) List(ctx context.Context, filter dnc.DNCProviderFilter) ([]*dnc.DNCProvider, error) {
	whereClause, args, err := r.buildProviderWhereClause(filter)
	if err != nil {
		return nil, err
	}

	orderBy := "priority ASC, name ASC"
	if filter.OrderBy != "" {
		direction := "ASC"
		if filter.OrderDesc {
			direction = "DESC"
		}
		orderBy = fmt.Sprintf("%s %s", filter.OrderBy, direction)
	}

	limit := ""
	if filter.Limit > 0 {
		limit = fmt.Sprintf("LIMIT %d", filter.Limit)
	}

	offset := ""
	if filter.Offset > 0 {
		offset = fmt.Sprintf("OFFSET %d", filter.Offset)
	}

	selectFields := `id, name, type, base_url, auth_type, api_key, update_frequency,
					 last_sync_at, next_sync_at, status, enabled, priority, retry_attempts,
					 timeout_seconds, rate_limit_per_min, last_sync_duration, last_sync_records,
					 last_error, error_count, success_count, config, created_at, updated_at,
					 created_by, updated_by`

	if !filter.IncludeConfig {
		selectFields = `id, name, type, base_url, auth_type, NULL as api_key, update_frequency,
						last_sync_at, next_sync_at, status, enabled, priority, retry_attempts,
						timeout_seconds, rate_limit_per_min, last_sync_duration, last_sync_records,
						last_error, error_count, success_count, '{}' as config, created_at, updated_at,
						created_by, updated_by`
	}

	query := fmt.Sprintf(`
		SELECT %s
		FROM dnc_providers
		WHERE deleted_at IS NULL %s
		ORDER BY %s
		%s %s
	`, selectFields, whereClause, orderBy, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.NewInternalError("failed to list DNC providers").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCProviders(rows)
}

// FindActive retrieves all currently active (enabled) providers
func (r *DNCProviderRepository) FindActive(ctx context.Context) ([]*dnc.DNCProvider, error) {
	query := `
		SELECT id, name, type, base_url, auth_type, api_key, update_frequency,
			   last_sync_at, next_sync_at, status, enabled, priority, retry_attempts,
			   timeout_seconds, rate_limit_per_min, last_sync_duration, last_sync_records,
			   last_error, error_count, success_count, config, created_at, updated_at,
			   created_by, updated_by
		FROM dnc_providers
		WHERE enabled = true 
		  AND status != 'inactive' 
		  AND deleted_at IS NULL
		ORDER BY priority ASC, name ASC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, errors.NewInternalError("failed to query active DNC providers").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCProviders(rows)
}

// FindByStatus retrieves providers by their operational status
func (r *DNCProviderRepository) FindByStatus(ctx context.Context, status dnc.ProviderStatus) ([]*dnc.DNCProvider, error) {
	query := `
		SELECT id, name, type, base_url, auth_type, api_key, update_frequency,
			   last_sync_at, next_sync_at, status, enabled, priority, retry_attempts,
			   timeout_seconds, rate_limit_per_min, last_sync_duration, last_sync_records,
			   last_error, error_count, success_count, config, created_at, updated_at,
			   created_by, updated_by
		FROM dnc_providers
		WHERE status = $1 AND deleted_at IS NULL
		ORDER BY priority ASC, name ASC
	`

	rows, err := r.db.Query(ctx, query, string(status))
	if err != nil {
		return nil, errors.NewInternalError("failed to query DNC providers by status").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCProviders(rows)
}

// FindNeedingSync retrieves providers that need synchronization
func (r *DNCProviderRepository) FindNeedingSync(ctx context.Context) ([]*dnc.DNCProvider, error) {
	query := `
		SELECT id, name, type, base_url, auth_type, api_key, update_frequency,
			   last_sync_at, next_sync_at, status, enabled, priority, retry_attempts,
			   timeout_seconds, rate_limit_per_min, last_sync_duration, last_sync_records,
			   last_error, error_count, success_count, config, created_at, updated_at,
			   created_by, updated_by
		FROM dnc_providers
		WHERE enabled = true 
		  AND deleted_at IS NULL
		  AND (
		    next_sync_at IS NULL 
		    OR next_sync_at <= NOW()
		    OR (last_sync_at IS NULL AND created_at < NOW() - INTERVAL '5 minutes')
		  )
		ORDER BY priority ASC, 
				 COALESCE(next_sync_at, created_at) ASC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, errors.NewInternalError("failed to query providers needing sync").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCProviders(rows)
}

// FindInError retrieves providers currently in error state
func (r *DNCProviderRepository) FindInError(ctx context.Context) ([]*dnc.DNCProvider, error) {
	query := `
		SELECT id, name, type, base_url, auth_type, api_key, update_frequency,
			   last_sync_at, next_sync_at, status, enabled, priority, retry_attempts,
			   timeout_seconds, rate_limit_per_min, last_sync_duration, last_sync_records,
			   last_error, error_count, success_count, config, created_at, updated_at,
			   created_by, updated_by
		FROM dnc_providers
		WHERE status = 'error' 
		  AND deleted_at IS NULL
		ORDER BY updated_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, errors.NewInternalError("failed to query providers in error").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCProviders(rows)
}

// UpdateSyncStatus updates the sync status and timestamps for a provider
func (r *DNCProviderRepository) UpdateSyncStatus(ctx context.Context, providerID uuid.UUID, status dnc.ProviderStatus, syncInfo *dnc.SyncInfo) error {
	query := `
		UPDATE dnc_providers 
		SET status = $2,
		    last_sync_at = $3,
		    next_sync_at = $4,
		    last_sync_duration = $5,
		    last_sync_records = $6,
		    last_error = $7,
		    error_count = CASE 
		      WHEN $2 = 'error' THEN error_count + 1
		      ELSE 0
		    END,
		    success_count = CASE 
		      WHEN $2 = 'active' THEN success_count + 1
		      ELSE success_count
		    END,
		    updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	var nextSyncAt *time.Time
	if syncInfo != nil && syncInfo.CompletedAt.After(time.Time{}) {
		// Calculate next sync time based on update frequency
		provider, err := r.GetByID(ctx, providerID)
		if err != nil {
			return err
		}
		next := syncInfo.CompletedAt.Add(provider.UpdateFrequency)
		nextSyncAt = &next
	}

	var lastError *string
	if syncInfo != nil && syncInfo.ErrorMsg != nil {
		lastError = syncInfo.ErrorMsg
	}

	var lastSyncAt *time.Time
	var lastSyncDuration *time.Duration
	var lastSyncRecords *int
	if syncInfo != nil {
		lastSyncAt = &syncInfo.CompletedAt
		lastSyncDuration = &syncInfo.Duration
		lastSyncRecords = &syncInfo.RecordCount
	}

	result, err := r.db.Exec(ctx, query,
		providerID,
		string(status),
		lastSyncAt,
		nextSyncAt,
		lastSyncDuration,
		lastSyncRecords,
		lastError,
	)

	if err != nil {
		return errors.NewInternalError("failed to update sync status").WithCause(err)
	}

	if result.RowsAffected() == 0 {
		return errors.NewNotFoundError("DNC provider not found")
	}

	return nil
}

// RecordSyncAttempt records a sync attempt with outcome
func (r *DNCProviderRepository) RecordSyncAttempt(ctx context.Context, providerID uuid.UUID, attempt *dnc.SyncAttempt) error {
	query := `
		INSERT INTO dnc_sync_attempts (
			id, provider_id, attempted_at, completed_at, status, records_read,
			records_added, records_updated, records_skipped, duration, error_msg,
			error_code, trigger_type, config, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
	`

	configJSON, err := json.Marshal(attempt.Config)
	if err != nil {
		return errors.NewInternalError("failed to marshal attempt config").WithCause(err)
	}

	metadataJSON, err := json.Marshal(attempt.Metadata)
	if err != nil {
		return errors.NewInternalError("failed to marshal attempt metadata").WithCause(err)
	}

	_, err = r.db.Exec(ctx, query,
		attempt.ID,
		attempt.ProviderID,
		attempt.AttemptedAt,
		attempt.CompletedAt,
		attempt.Status,
		attempt.RecordsRead,
		attempt.RecordsAdded,
		attempt.RecordsUpdated,
		attempt.RecordsSkipped,
		attempt.Duration,
		attempt.ErrorMsg,
		attempt.ErrorCode,
		attempt.TriggerType,
		configJSON,
		metadataJSON,
	)

	if err != nil {
		return errors.NewInternalError("failed to record sync attempt").WithCause(err)
	}

	return nil
}

// GetSyncHistory retrieves sync history for a provider
func (r *DNCProviderRepository) GetSyncHistory(ctx context.Context, providerID uuid.UUID, limit int) ([]*dnc.SyncAttempt, error) {
	query := `
		SELECT id, provider_id, attempted_at, completed_at, status, records_read,
			   records_added, records_updated, records_skipped, duration, error_msg,
			   error_code, trigger_type, config, metadata
		FROM dnc_sync_attempts
		WHERE provider_id = $1
		ORDER BY attempted_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, providerID, limit)
	if err != nil {
		return nil, errors.NewInternalError("failed to get sync history").WithCause(err)
	}
	defer rows.Close()

	var attempts []*dnc.SyncAttempt
	for rows.Next() {
		attempt, err := r.scanSyncAttempt(rows)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, attempt)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("error iterating sync attempts").WithCause(err)
	}

	return attempts, nil
}

// UpdateConfig updates provider configuration without affecting sync state
func (r *DNCProviderRepository) UpdateConfig(ctx context.Context, providerID uuid.UUID, config map[string]string) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return errors.NewInternalError("failed to marshal config").WithCause(err)
	}

	query := `
		UPDATE dnc_providers 
		SET config = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.Exec(ctx, query, providerID, configJSON)
	if err != nil {
		return errors.NewInternalError("failed to update provider config").WithCause(err)
	}

	if result.RowsAffected() == 0 {
		return errors.NewNotFoundError("DNC provider not found")
	}

	return nil
}

// UpdateAuth updates authentication credentials securely
func (r *DNCProviderRepository) UpdateAuth(ctx context.Context, providerID uuid.UUID, authType dnc.AuthType, credentials *string) error {
	query := `
		UPDATE dnc_providers 
		SET auth_type = $2, api_key = $3, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.Exec(ctx, query, providerID, string(authType), credentials)
	if err != nil {
		return errors.NewInternalError("failed to update provider auth").WithCause(err)
	}

	if result.RowsAffected() == 0 {
		return errors.NewNotFoundError("DNC provider not found")
	}

	return nil
}

// GetHealthStatus retrieves health status for all providers
func (r *DNCProviderRepository) GetHealthStatus(ctx context.Context) ([]*dnc.ProviderHealthStatus, error) {
	query := `
		SELECT 
			p.id,
			p.name,
			p.status,
			p.last_sync_at,
			p.last_error,
			p.error_count,
			p.success_count,
			CASE 
				WHEN p.status = 'active' AND p.error_count = 0 THEN 'healthy'
				WHEN p.status = 'active' AND p.error_count > 0 THEN 'degraded'
				ELSE 'unhealthy'
			END as overall_health,
			CASE 
				WHEN p.last_sync_at IS NOT NULL 
				AND p.last_sync_at > NOW() - (p.update_frequency * 2)
				THEN true
				ELSE false
			END as is_responding,
			COALESCE(p.last_sync_duration, INTERVAL '0') as response_time,
			CASE 
				WHEN p.error_count = 0 THEN 1.0
				ELSE GREATEST(0.0, 1.0 - (p.error_count::float / GREATEST(p.success_count + p.error_count, 1)))
			END as recent_success_rate
		FROM dnc_providers p
		WHERE p.deleted_at IS NULL
		ORDER BY p.priority ASC, p.name ASC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, errors.NewInternalError("failed to get health status").WithCause(err)
	}
	defer rows.Close()

	var statuses []*dnc.ProviderHealthStatus
	for rows.Next() {
		var status dnc.ProviderHealthStatus
		var responseTimeInterval sql.NullString
		var lastSyncAt sql.NullTime

		err := rows.Scan(
			&status.ProviderID,
			&status.ProviderName,
			&status.OverallHealth,
			&lastSyncAt,
			&status.ErrorCount,
			&status.WarningCount,
			&status.RecentErrorCount,
			&status.OverallHealth,
			&status.IsResponding,
			&responseTimeInterval,
			&status.RecentSuccessRate,
		)

		if err != nil {
			return nil, errors.NewInternalError("failed to scan health status").WithCause(err)
		}

		if lastSyncAt.Valid {
			status.LastCheckAt = lastSyncAt.Time
		}

		if responseTimeInterval.Valid {
			// Parse PostgreSQL interval to Go duration
			if duration, err := time.ParseDuration(responseTimeInterval.String); err == nil {
				status.ResponseTime = duration
			}
		}

		// Set connectivity and authentication based on recent sync status
		status.ConnectivityOK = status.IsResponding
		status.AuthenticationOK = status.IsResponding && status.RecentErrorCount == 0

		statuses = append(statuses, &status)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("error iterating health statuses").WithCause(err)
	}

	return statuses, nil
}

// GetProviderMetrics retrieves performance metrics for a provider
func (r *DNCProviderRepository) GetProviderMetrics(ctx context.Context, providerID uuid.UUID, timeRange dnc.TimeRange) (*dnc.ProviderMetrics, error) {
	metrics := &dnc.ProviderMetrics{
		ProviderID: providerID,
		TimeRange:  timeRange,
	}

	// Get sync metrics
	err := r.db.QueryRow(ctx, `
		SELECT 
			COUNT(*) as total_syncs,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as successful_syncs,
			COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed_syncs,
			COALESCE(AVG(EXTRACT(EPOCH FROM duration)), 0) as avg_duration_seconds,
			COALESCE(MIN(EXTRACT(EPOCH FROM duration)), 0) as min_duration_seconds,
			COALESCE(MAX(EXTRACT(EPOCH FROM duration)), 0) as max_duration_seconds,
			COALESCE(SUM(records_read), 0) as total_records,
			COALESCE(SUM(records_added), 0) as added_records,
			COALESCE(SUM(records_updated), 0) as updated_records,
			COALESCE(SUM(records_skipped), 0) as skipped_records
		FROM dnc_sync_attempts
		WHERE provider_id = $1
		  AND attempted_at BETWEEN $2 AND $3
	`, providerID, timeRange.Start, timeRange.End).Scan(
		&metrics.TotalSyncs,
		&metrics.SuccessfulSyncs,
		&metrics.FailedSyncs,
		&metrics.AvgDuration,
		&metrics.MinDuration,
		&metrics.MaxDuration,
		&metrics.TotalRecords,
		&metrics.AddedRecords,
		&metrics.UpdatedRecords,
		&metrics.SkippedRecords,
	)

	if err != nil {
		return nil, errors.NewInternalError("failed to get provider metrics").WithCause(err)
	}

	// Calculate success rate
	if metrics.TotalSyncs > 0 {
		metrics.SuccessRate = float64(metrics.SuccessfulSyncs) / float64(metrics.TotalSyncs)
	}

	// Calculate throughput (records per second average)
	if metrics.AvgDuration.Seconds() > 0 {
		avgRecordsPerSync := float64(metrics.TotalRecords) / float64(metrics.TotalSyncs)
		metrics.Throughput = avgRecordsPerSync / metrics.AvgDuration.Seconds()
	}

	// Get top errors
	errorRows, err := r.db.Query(ctx, `
		SELECT error_code, error_msg, COUNT(*) as count, MAX(attempted_at) as last_occurred
		FROM dnc_sync_attempts
		WHERE provider_id = $1
		  AND attempted_at BETWEEN $2 AND $3
		  AND error_code IS NOT NULL
		GROUP BY error_code, error_msg
		ORDER BY count DESC
		LIMIT 5
	`, providerID, timeRange.Start, timeRange.End)

	if err != nil {
		return nil, errors.NewInternalError("failed to get error summary").WithCause(err)
	}
	defer errorRows.Close()

	for errorRows.Next() {
		var errorSummary dnc.ErrorSummary
		var errorCode, errorMsg sql.NullString

		err := errorRows.Scan(&errorCode, &errorMsg, &errorSummary.Count, &errorSummary.LastOccurred)
		if err != nil {
			return nil, errors.NewInternalError("failed to scan error summary").WithCause(err)
		}

		if errorCode.Valid {
			errorSummary.ErrorCode = errorCode.String
		}
		if errorMsg.Valid {
			errorSummary.ErrorMsg = errorMsg.String
		}

		metrics.TopErrors = append(metrics.TopErrors, errorSummary)
	}

	return metrics, nil
}

// UpdateHealthCheck updates the health check status for a provider
func (r *DNCProviderRepository) UpdateHealthCheck(ctx context.Context, providerID uuid.UUID, health *dnc.HealthCheckResult) error {
	query := `
		INSERT INTO dnc_provider_health_checks (
			provider_id, checked_at, is_healthy, response_time, status_code,
			error_msg, connectivity, authentication, data_available, rate_limit, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
	`

	metadataJSON, err := json.Marshal(health.Metadata)
	if err != nil {
		return errors.NewInternalError("failed to marshal health check metadata").WithCause(err)
	}

	_, err = r.db.Exec(ctx, query,
		providerID,
		health.CheckedAt,
		health.IsHealthy,
		health.ResponseTime,
		health.StatusCode,
		health.ErrorMsg,
		health.Connectivity,
		health.Authentication,
		health.DataAvailable,
		health.RateLimit,
		metadataJSON,
	)

	if err != nil {
		return errors.NewInternalError("failed to update health check").WithCause(err)
	}

	return nil
}

// BulkUpdateStatus updates status for multiple providers
func (r *DNCProviderRepository) BulkUpdateStatus(ctx context.Context, updates []dnc.ProviderStatusUpdate) error {
	if len(updates) == 0 {
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

	for _, update := range updates {
		query := `
			UPDATE dnc_providers 
			SET status = $2, updated_at = NOW(), updated_by = $3
			WHERE id = $1 AND deleted_at IS NULL
		`

		_, err = pgxTx.tx.Exec(ctx, query, update.ProviderID, string(update.Status), update.UpdatedBy)
		if err != nil {
			return errors.NewInternalError("failed to bulk update provider status").WithCause(err)
		}
	}

	return tx.Commit()
}

// BulkUpdateConfig updates configuration for multiple providers
func (r *DNCProviderRepository) BulkUpdateConfig(ctx context.Context, updates []dnc.ProviderConfigUpdate) error {
	if len(updates) == 0 {
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

	for _, update := range updates {
		configJSON, err := json.Marshal(update.Config)
		if err != nil {
			return errors.NewInternalError("failed to marshal config").WithCause(err)
		}

		query := `
			UPDATE dnc_providers 
			SET config = $2, updated_at = NOW(), updated_by = $3
			WHERE id = $1 AND deleted_at IS NULL
		`

		_, err = pgxTx.tx.Exec(ctx, query, update.ProviderID, configJSON, update.UpdatedBy)
		if err != nil {
			return errors.NewInternalError("failed to bulk update provider config").WithCause(err)
		}
	}

	return tx.Commit()
}

// GetStats returns repository performance and usage statistics
func (r *DNCProviderRepository) GetStats(ctx context.Context) (*dnc.DNCProviderStats, error) {
	stats := &dnc.DNCProviderStats{
		CollectedAt: time.Now(),
	}

	// Total providers
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM dnc_providers WHERE deleted_at IS NULL
	`).Scan(&stats.TotalProviders)
	if err != nil {
		return nil, errors.NewInternalError("failed to get total providers count").WithCause(err)
	}

	// Providers by type
	stats.ProvidersByType = make(map[string]int64)
	rows, err := r.db.Query(ctx, `
		SELECT type, COUNT(*) 
		FROM dnc_providers 
		WHERE deleted_at IS NULL 
		GROUP BY type
	`)
	if err != nil {
		return nil, errors.NewInternalError("failed to get providers by type").WithCause(err)
	}
	defer rows.Close()

	for rows.Next() {
		var providerType string
		var count int64
		if err := rows.Scan(&providerType, &count); err != nil {
			return nil, errors.NewInternalError("failed to scan provider type stats").WithCause(err)
		}
		stats.ProvidersByType[providerType] = count
	}

	// Providers by status
	stats.ProvidersByStatus = make(map[string]int64)
	statusRows, err := r.db.Query(ctx, `
		SELECT status, COUNT(*) 
		FROM dnc_providers 
		WHERE deleted_at IS NULL 
		GROUP BY status
	`)
	if err != nil {
		return nil, errors.NewInternalError("failed to get providers by status").WithCause(err)
	}
	defer statusRows.Close()

	for statusRows.Next() {
		var status string
		var count int64
		if err := statusRows.Scan(&status, &count); err != nil {
			return nil, errors.NewInternalError("failed to scan provider status stats").WithCause(err)
		}
		stats.ProvidersByStatus[status] = count
	}

	// Active and healthy providers
	err = r.db.QueryRow(ctx, `
		SELECT 
			COUNT(CASE WHEN enabled = true THEN 1 END) as active,
			COUNT(CASE WHEN status = 'active' AND error_count = 0 THEN 1 END) as healthy
		FROM dnc_providers WHERE deleted_at IS NULL
	`).Scan(&stats.ActiveProviders, &stats.HealthyProviders)
	if err != nil {
		return nil, errors.NewInternalError("failed to get active/healthy counts").WithCause(err)
	}

	// Sync performance metrics
	err = r.db.QueryRow(ctx, `
		SELECT 
			COALESCE(AVG(EXTRACT(EPOCH FROM last_sync_duration)), 0) as avg_sync_duration_seconds,
			COALESCE(AVG(CASE WHEN success_count + error_count > 0 
				THEN success_count::float / (success_count + error_count) 
				ELSE 0 END), 0) as avg_success_rate
		FROM dnc_providers 
		WHERE deleted_at IS NULL
	`).Scan(&stats.AvgSyncDuration, &stats.AvgSuccessRate)
	if err != nil {
		return nil, errors.NewInternalError("failed to get sync performance metrics").WithCause(err)
	}

	// Today's sync stats
	err = r.db.QueryRow(ctx, `
		SELECT 
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as successful,
			COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed
		FROM dnc_sync_attempts 
		WHERE attempted_at >= CURRENT_DATE
	`).Scan(&stats.TotalSyncsToday, &stats.FailedSyncsToday)
	if err != nil {
		return nil, errors.NewInternalError("failed to get today's sync stats").WithCause(err)
	}

	// Error states
	err = r.db.QueryRow(ctx, `
		SELECT 
			COUNT(CASE WHEN status = 'error' THEN 1 END) as in_error,
			COUNT(CASE WHEN next_sync_at IS NOT NULL AND next_sync_at < NOW() THEN 1 END) as overdue
		FROM dnc_providers 
		WHERE deleted_at IS NULL
	`).Scan(&stats.ProvidersInError, &stats.ProvidersOverdue)
	if err != nil {
		return nil, errors.NewInternalError("failed to get error state counts").WithCause(err)
	}

	return stats, nil
}

// Vacuum performs database maintenance operations
func (r *DNCProviderRepository) Vacuum(ctx context.Context) error {
	queries := []string{
		"VACUUM ANALYZE dnc_providers",
		"VACUUM ANALYZE dnc_sync_attempts",
		"VACUUM ANALYZE dnc_provider_health_checks",
		"REINDEX INDEX idx_dnc_providers_type",
		"REINDEX INDEX idx_dnc_providers_status",
		"REINDEX INDEX idx_dnc_providers_next_sync",
	}

	for _, query := range queries {
		_, err := r.db.Exec(ctx, query)
		if err != nil {
			return errors.NewInternalError("failed to vacuum DNC providers").WithCause(err)
		}
	}

	return nil
}

// ValidateIntegrity performs integrity checks on provider data
func (r *DNCProviderRepository) ValidateIntegrity(ctx context.Context) (*dnc.ProviderIntegrityReport, error) {
	report := &dnc.ProviderIntegrityReport{
		GeneratedAt: time.Now(),
		IsHealthy:   true,
	}

	startTime := time.Now()

	// Check total providers
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM dnc_providers WHERE deleted_at IS NULL
	`).Scan(&report.TotalProviders)
	if err != nil {
		return nil, errors.NewInternalError("failed to count total providers").WithCause(err)
	}

	// Validate URLs
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM dnc_providers 
		WHERE deleted_at IS NULL 
		  AND base_url ~ '^https?://.+'
	`).Scan(&report.ValidProviders)
	if err != nil {
		return nil, errors.NewInternalError("failed to count valid providers").WithCause(err)
	}

	report.InvalidProviders = report.TotalProviders - report.ValidProviders

	// Check for duplicate names
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM (
			SELECT name
			FROM dnc_providers
			WHERE deleted_at IS NULL
			GROUP BY name
			HAVING COUNT(*) > 1
		) duplicates
	`).Scan(&report.DuplicateNames)
	if err != nil {
		return nil, errors.NewInternalError("failed to count duplicate names").WithCause(err)
	}

	// Check stale providers (no sync for 2x update frequency)
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM dnc_providers
		WHERE deleted_at IS NULL
		  AND enabled = true
		  AND (
		    last_sync_at IS NULL
		    OR last_sync_at < NOW() - (update_frequency * 2)
		  )
	`).Scan(&report.StaleProviders)
	if err != nil {
		return nil, errors.NewInternalError("failed to count stale providers").WithCause(err)
	}

	// Overall health assessment
	if report.InvalidProviders > 0 {
		report.IsHealthy = false
		report.CriticalIssues = append(report.CriticalIssues, 
			fmt.Sprintf("Found %d providers with invalid URLs", report.InvalidProviders))
	}

	if report.DuplicateNames > 0 {
		report.Warnings = append(report.Warnings, 
			fmt.Sprintf("Found %d duplicate provider names", report.DuplicateNames))
	}

	if report.StaleProviders > 0 {
		report.Warnings = append(report.Warnings, 
			fmt.Sprintf("Found %d stale providers (not synced recently)", report.StaleProviders))
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
func (r *DNCProviderRepository) BeginTx(ctx context.Context) (dnc.Transaction, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, errors.NewInternalError("failed to begin transaction").WithCause(err)
	}

	return &PgxTransaction{tx: tx, ctx: ctx}, nil
}

// WithTx executes a function within a database transaction
func (r *DNCProviderRepository) WithTx(ctx context.Context, fn func(tx dnc.Transaction) error) error {
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

// scanDNCProvider scans a single DNC provider from a database row
func (r *DNCProviderRepository) scanDNCProvider(row pgx.Row) (*dnc.DNCProvider, error) {
	var provider dnc.DNCProvider
	var providerType, authType, status string
	var configJSON []byte

	err := row.Scan(
		&provider.ID,
		&provider.Name,
		&providerType,
		&provider.BaseURL,
		&authType,
		&provider.APIKey,
		&provider.UpdateFrequency,
		&provider.LastSyncAt,
		&provider.NextSyncAt,
		&status,
		&provider.Enabled,
		&provider.Priority,
		&provider.RetryAttempts,
		&provider.TimeoutSeconds,
		&provider.RateLimitPerMin,
		&provider.LastSyncDuration,
		&provider.LastSyncRecords,
		&provider.LastError,
		&provider.ErrorCount,
		&provider.SuccessCount,
		&configJSON,
		&provider.CreatedAt,
		&provider.UpdatedAt,
		&provider.CreatedBy,
		&provider.UpdatedBy,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.NewNotFoundError("DNC provider not found")
		}
		return nil, errors.NewInternalError("failed to scan DNC provider").WithCause(err)
	}

	// Convert string fields to enums
	provider.Type = dnc.ProviderType(providerType)
	provider.AuthType = dnc.AuthType(authType)
	provider.Status = dnc.ProviderStatus(status)

	// Unmarshal config
	if configJSON != nil {
		err = json.Unmarshal(configJSON, &provider.Config)
		if err != nil {
			return nil, errors.NewInternalError("failed to unmarshal config").WithCause(err)
		}
	}

	return &provider, nil
}

// scanDNCProviders scans multiple DNC providers from database rows
func (r *DNCProviderRepository) scanDNCProviders(rows pgx.Rows) ([]*dnc.DNCProvider, error) {
	var providers []*dnc.DNCProvider

	for rows.Next() {
		provider, err := r.scanDNCProvider(rows)
		if err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("error iterating DNC providers").WithCause(err)
	}

	return providers, nil
}

// scanSyncAttempt scans a sync attempt from a database row
func (r *DNCProviderRepository) scanSyncAttempt(row pgx.Row) (*dnc.SyncAttempt, error) {
	var attempt dnc.SyncAttempt
	var configJSON, metadataJSON []byte

	err := row.Scan(
		&attempt.ID,
		&attempt.ProviderID,
		&attempt.AttemptedAt,
		&attempt.CompletedAt,
		&attempt.Status,
		&attempt.RecordsRead,
		&attempt.RecordsAdded,
		&attempt.RecordsUpdated,
		&attempt.RecordsSkipped,
		&attempt.Duration,
		&attempt.ErrorMsg,
		&attempt.ErrorCode,
		&attempt.TriggerType,
		&configJSON,
		&metadataJSON,
	)

	if err != nil {
		return nil, errors.NewInternalError("failed to scan sync attempt").WithCause(err)
	}

	// Unmarshal JSON fields
	if configJSON != nil {
		err = json.Unmarshal(configJSON, &attempt.Config)
		if err != nil {
			return nil, errors.NewInternalError("failed to unmarshal attempt config").WithCause(err)
		}
	}

	if metadataJSON != nil {
		err = json.Unmarshal(metadataJSON, &attempt.Metadata)
		if err != nil {
			return nil, errors.NewInternalError("failed to unmarshal attempt metadata").WithCause(err)
		}
	}

	return &attempt, nil
}

// buildProviderWhereClause builds a WHERE clause and arguments from a provider filter
func (r *DNCProviderRepository) buildProviderWhereClause(filter dnc.DNCProviderFilter) (string, []interface{}, error) {
	conditions := make([]string, 0)
	args := make([]interface{}, 0)
	argIndex := 1

	// Type filters
	if len(filter.Types) > 0 {
		typeStrs := make([]string, len(filter.Types))
		for i, t := range filter.Types {
			typeStrs[i] = string(t)
		}
		conditions = append(conditions, fmt.Sprintf("type = ANY($%d)", argIndex))
		args = append(args, typeStrs)
		argIndex++
	}

	// Status filters
	if len(filter.Statuses) > 0 {
		statusStrs := make([]string, len(filter.Statuses))
		for i, s := range filter.Statuses {
			statusStrs[i] = string(s)
		}
		conditions = append(conditions, fmt.Sprintf("status = ANY($%d)", argIndex))
		args = append(args, statusStrs)
		argIndex++
	}

	// Name filters
	if len(filter.Names) > 0 {
		conditions = append(conditions, fmt.Sprintf("name = ANY($%d)", argIndex))
		args = append(args, filter.Names)
		argIndex++
	}

	if filter.NamePattern != nil {
		conditions = append(conditions, fmt.Sprintf("name LIKE $%d", argIndex))
		args = append(args, *filter.NamePattern)
		argIndex++
	}

	// Status-based filters
	if filter.OnlyActive != nil && *filter.OnlyActive {
		conditions = append(conditions, "enabled = true AND status != 'inactive'")
	}

	if filter.OnlyEnabled != nil && *filter.OnlyEnabled {
		conditions = append(conditions, "enabled = true")
	}

	if filter.OnlyInError != nil && *filter.OnlyInError {
		conditions = append(conditions, "status = 'error'")
	}

	if filter.NeedsSync != nil && *filter.NeedsSync {
		conditions = append(conditions, "(next_sync_at IS NULL OR next_sync_at <= NOW())")
	}

	// Time range filters
	if filter.CreatedAfter != nil {
		conditions = append(conditions, fmt.Sprintf("created_at > $%d", argIndex))
		args = append(args, *filter.CreatedAfter)
		argIndex++
	}

	if filter.CreatedBefore != nil {
		conditions = append(conditions, fmt.Sprintf("created_at < $%d", argIndex))
		args = append(args, *filter.CreatedBefore)
		argIndex++
	}

	if filter.LastSyncAfter != nil {
		conditions = append(conditions, fmt.Sprintf("last_sync_at > $%d", argIndex))
		args = append(args, *filter.LastSyncAfter)
		argIndex++
	}

	if filter.LastSyncBefore != nil {
		conditions = append(conditions, fmt.Sprintf("last_sync_at < $%d", argIndex))
		args = append(args, *filter.LastSyncBefore)
		argIndex++
	}

	// Performance filters
	if filter.MinSuccessRate != nil {
		conditions = append(conditions, fmt.Sprintf(`
			CASE WHEN success_count + error_count > 0 
			THEN success_count::float / (success_count + error_count) 
			ELSE 0 END >= $%d`, argIndex))
		args = append(args, *filter.MinSuccessRate)
		argIndex++
	}

	if filter.MaxErrorCount != nil {
		conditions = append(conditions, fmt.Sprintf("error_count <= $%d", argIndex))
		args = append(args, *filter.MaxErrorCount)
		argIndex++
	}

	// Search text
	if filter.SearchText != nil {
		conditions = append(conditions, fmt.Sprintf(`
			(name ILIKE $%d OR base_url ILIKE $%d OR config::text ILIKE $%d)`, 
			argIndex, argIndex, argIndex))
		searchPattern := "%" + *filter.SearchText + "%"
		args = append(args, searchPattern)
		argIndex++
	}

	// Build final WHERE clause
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "AND " + strings.Join(conditions, " AND ")
	}

	return whereClause, args, nil
}