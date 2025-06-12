package database

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// BaseRepository provides common database operations with advanced features
type BaseRepository struct {
	pool   *ConnectionPool
	logger *zap.Logger
	cache  CacheInterface
	tracer TracerInterface
}

// CacheInterface defines caching operations
type CacheInterface interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context, pattern string) error
}

// TracerInterface defines distributed tracing operations
type TracerInterface interface {
	StartSpan(ctx context.Context, name string) (context.Context, func())
}

// QueryBuilder helps construct complex SQL queries safely
type QueryBuilder struct {
	table      string
	schema     string
	selections []string
	joins      []string
	conditions []string
	args       []interface{}
	argCounter int
	groupBy    []string
	orderBy    []string
	limit      *int
	offset     *int
}

// NewBaseRepository creates a new base repository
func NewBaseRepository(pool *ConnectionPool, logger *zap.Logger) *BaseRepository {
	return &BaseRepository{
		pool:   pool,
		logger: logger,
	}
}

// WithCache adds caching capability
func (r *BaseRepository) WithCache(cache CacheInterface) *BaseRepository {
	r.cache = cache
	return r
}

// WithTracer adds distributed tracing
func (r *BaseRepository) WithTracer(tracer TracerInterface) *BaseRepository {
	r.tracer = tracer
	return r
}

// NewQueryBuilder creates a new query builder
func (r *BaseRepository) NewQueryBuilder(schema, table string) *QueryBuilder {
	return &QueryBuilder{
		schema:     schema,
		table:      table,
		selections: []string{"*"},
		argCounter: 0,
	}
}

// Select sets the columns to select
func (qb *QueryBuilder) Select(columns ...string) *QueryBuilder {
	qb.selections = columns
	return qb
}

// Where adds a WHERE condition
func (qb *QueryBuilder) Where(condition string, args ...interface{}) *QueryBuilder {
	placeholders := make([]string, len(args))
	for i, arg := range args {
		qb.argCounter++
		placeholders[i] = fmt.Sprintf("$%d", qb.argCounter)
		qb.args = append(qb.args, arg)
	}

	// Replace ? with numbered placeholders
	for _, placeholder := range placeholders {
		condition = strings.Replace(condition, "?", placeholder, 1)
	}

	qb.conditions = append(qb.conditions, condition)
	return qb
}

// Join adds a JOIN clause
func (qb *QueryBuilder) Join(joinType, table, condition string) *QueryBuilder {
	join := fmt.Sprintf("%s JOIN %s ON %s", joinType, table, condition)
	qb.joins = append(qb.joins, join)
	return qb
}

// GroupBy adds GROUP BY columns
func (qb *QueryBuilder) GroupBy(columns ...string) *QueryBuilder {
	qb.groupBy = columns
	return qb
}

// OrderBy adds ORDER BY columns
func (qb *QueryBuilder) OrderBy(column string, desc bool) *QueryBuilder {
	order := column
	if desc {
		order += " DESC"
	}
	qb.orderBy = append(qb.orderBy, order)
	return qb
}

// Limit sets the LIMIT
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.limit = &limit
	return qb
}

// Offset sets the OFFSET
func (qb *QueryBuilder) Offset(offset int) *QueryBuilder {
	qb.offset = &offset
	return qb
}

// Build constructs the final SQL query
func (qb *QueryBuilder) Build() (string, []interface{}) {
	var query strings.Builder

	// SELECT
	query.WriteString("SELECT ")
	query.WriteString(strings.Join(qb.selections, ", "))

	// FROM
	query.WriteString(" FROM ")
	if qb.schema != "" {
		query.WriteString(qb.schema)
		query.WriteString(".")
	}
	query.WriteString(qb.table)

	// JOIN
	for _, join := range qb.joins {
		query.WriteString(" ")
		query.WriteString(join)
	}

	// WHERE
	if len(qb.conditions) > 0 {
		query.WriteString(" WHERE ")
		query.WriteString(strings.Join(qb.conditions, " AND "))
	}

	// GROUP BY
	if len(qb.groupBy) > 0 {
		query.WriteString(" GROUP BY ")
		query.WriteString(strings.Join(qb.groupBy, ", "))
	}

	// ORDER BY
	if len(qb.orderBy) > 0 {
		query.WriteString(" ORDER BY ")
		query.WriteString(strings.Join(qb.orderBy, ", "))
	}

	// LIMIT
	if qb.limit != nil {
		query.WriteString(fmt.Sprintf(" LIMIT %d", *qb.limit))
	}

	// OFFSET
	if qb.offset != nil {
		query.WriteString(fmt.Sprintf(" OFFSET %d", *qb.offset))
	}

	return query.String(), qb.args
}

// ExecuteQuery executes a query with caching and tracing
func (r *BaseRepository) ExecuteQuery(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	// Start tracing span
	if r.tracer != nil {
		var finish func()
		ctx, finish = r.tracer.StartSpan(ctx, "database.query")
		defer finish()
	}

	// Log query execution
	r.logger.Debug("executing query",
		zap.String("query", query),
		zap.Any("args", args))

	// Get appropriate connection
	conn := r.pool.GetReadConnection(false)

	// Execute query with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	start := time.Now()
	rows, err := conn.Query(ctx, query, args...)
	duration := time.Since(start)

	// Record metrics
	if err != nil {
		r.logger.Error("query failed",
			zap.String("query", query),
			zap.Error(err),
			zap.Duration("duration", duration))
		return nil, err
	}

	r.logger.Debug("query completed",
		zap.Duration("duration", duration))

	return &pgxRowsAdapterImpl{rows: rows}, nil
}

// ExecuteQueryRow executes a query returning a single row
func (r *BaseRepository) ExecuteQueryRow(ctx context.Context, query string, args ...interface{}) Row {
	// Start tracing span
	if r.tracer != nil {
		var finish func()
		ctx, finish = r.tracer.StartSpan(ctx, "database.query_row")
		defer finish()
	}

	// Get appropriate connection
	conn := r.pool.GetReadConnection(false)

	// Execute query with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	row := conn.QueryRow(ctx, query, args...)
	return &pgxRowAdapterImpl{row: row}
}

// ExecuteCommand executes a command (INSERT, UPDATE, DELETE)
func (r *BaseRepository) ExecuteCommand(ctx context.Context, query string, args ...interface{}) (Result, error) {
	// Start tracing span
	if r.tracer != nil {
		var finish func()
		ctx, finish = r.tracer.StartSpan(ctx, "database.command")
		defer finish()
	}

	// Commands always go to primary
	conn := r.pool.GetPrimary()

	// Execute command with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	start := time.Now()
	tag, err := conn.Exec(ctx, query, args...)
	duration := time.Since(start)

	if err != nil {
		r.logger.Error("command failed",
			zap.String("query", query),
			zap.Error(err),
			zap.Duration("duration", duration))
		return nil, err
	}

	r.logger.Debug("command completed",
		zap.String("command", tag.String()),
		zap.Duration("duration", duration))

	return &pgxResultAdapterImpl{tag: tag}, nil
}

// Transaction executes operations within a transaction
func (r *BaseRepository) Transaction(ctx context.Context, fn TransactionFunc) error {
	// Start tracing span
	if r.tracer != nil {
		var finish func()
		ctx, finish = r.tracer.StartSpan(ctx, "database.transaction")
		defer finish()
	}

	return r.pool.Transaction(ctx, func(pgxTx pgx.Tx) error {
		// Create transaction context with abstracted tx
		tx := &pgxTxAdapterImpl{tx: pgxTx}
		txCtx := context.WithValue(ctx, "tx", tx)
		return fn(txCtx, tx)
	})
}

// GetByID retrieves an entity by ID with caching
func (r *BaseRepository) GetByID(ctx context.Context, schema, table string, id uuid.UUID, dest interface{}) error {
	// Check cache first
	if r.cache != nil {
		cacheKey := fmt.Sprintf("%s:%s:%s", schema, table, id)
		if data, err := r.cache.Get(ctx, cacheKey); err == nil {
			if err := json.Unmarshal(data, dest); err == nil {
				r.logger.Debug("cache hit", zap.String("key", cacheKey))
				return nil
			}
		}
	}

	// Query database
	query := fmt.Sprintf("SELECT * FROM %s.%s WHERE id = $1 AND deleted_at IS NULL", schema, table)
	row := r.ExecuteQueryRow(ctx, query, id)

	// Scan the row into the destination struct
	// Note: The caller must ensure dest is a pointer to a scannable type
	err := row.Scan(dest)
	if err != nil {
		if err == pgx.ErrNoRows {
			return NoRowsError
		}
		return fmt.Errorf("failed to scan row: %w", err)
	}

	// Update cache with TTL
	if r.cache != nil {
		if data, err := json.Marshal(dest); err == nil {
			cacheKey := fmt.Sprintf("%s:%s:%s", schema, table, id)
			// Use reasonable cache TTL - 5 minutes for frequently changing data
			if err := r.cache.Set(ctx, cacheKey, data, 5*time.Minute); err != nil {
				r.logger.Warn("failed to update cache",
					zap.String("key", cacheKey),
					zap.Error(err))
			}
		}
	}

	return nil
}

// BatchInsert performs efficient batch insertion
func (r *BaseRepository) BatchInsert(ctx context.Context, schema, table string, columns []string, values [][]interface{}) error {
	if len(values) == 0 {
		return nil
	}

	// Start transaction
	return r.Transaction(ctx, func(ctx context.Context, tx Tx) error {
		// For batch inserts, we need to use the underlying pgx transaction
		// This is a limitation of the abstraction - COPY operations are pgx-specific
		pgxTx, ok := tx.(*pgxTxAdapterImpl)
		if !ok {
			return fmt.Errorf("batch insert requires pgx transaction")
		}

		// Use COPY directly without temp table for better performance
		copyFrom := pgx.CopyFromSlice(len(values), func(i int) ([]interface{}, error) {
			return values[i], nil
		})

		// Direct copy to target table
		targetTable := pgx.Identifier{schema, table}
		_, err := pgxTx.tx.CopyFrom(ctx, targetTable, columns, copyFrom)
		if err != nil {
			return fmt.Errorf("failed to batch insert: %w", err)
		}

		return nil
	})
}

// BulkUpdate performs efficient bulk updates
func (r *BaseRepository) BulkUpdate(ctx context.Context, schema, table string, updates map[uuid.UUID]map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	return r.Transaction(ctx, func(ctx context.Context, tx Tx) error {
		// Collect all unique field names to ensure consistent ordering
		fieldSet := make(map[string]bool)
		for _, fields := range updates {
			for field := range fields {
				fieldSet[field] = true
			}
		}

		// Convert to sorted slice for consistent ordering
		var fieldNames []string
		for field := range fieldSet {
			fieldNames = append(fieldNames, field)
		}
		sort.Strings(fieldNames)

		if len(fieldNames) == 0 {
			return fmt.Errorf("no fields to update")
		}

		// Build CTE values and collect arguments
		var cteRows []string
		var args []interface{}
		argCounter := 0

		for id, fields := range updates {
			// Add ID as first argument
			argCounter++
			args = append(args, id)
			rowValues := []string{fmt.Sprintf("$%d", argCounter)}

			// Add field values in consistent order
			for _, fieldName := range fieldNames {
				if value, exists := fields[fieldName]; exists {
					argCounter++
					args = append(args, value)
					rowValues = append(rowValues, fmt.Sprintf("$%d", argCounter))
				} else {
					// Use NULL for missing fields
					rowValues = append(rowValues, "NULL")
				}
			}

			cteRows = append(cteRows, fmt.Sprintf("(%s)", strings.Join(rowValues, ", ")))
		}

		// Build SET clause
		var setClauses []string
		for _, fieldName := range fieldNames {
			setClauses = append(setClauses, fmt.Sprintf("%s = COALESCE(u.%s, t.%s)", fieldName, fieldName, fieldName))
		}

		// Build the complete query
		updateQuery := fmt.Sprintf(`
			WITH updates (id, %s) AS (
				VALUES %s
			)
			UPDATE %s.%s t
			SET %s, updated_at = NOW()
			FROM updates u
			WHERE t.id = u.id
		`, strings.Join(fieldNames, ", "), strings.Join(cteRows, ", "), schema, table, strings.Join(setClauses, ", "))

		_, err := tx.Exec(ctx, updateQuery, args...)
		return err
	})
}

// StreamQuery executes a query and streams results
func (r *BaseRepository) StreamQuery(ctx context.Context, query string, args []interface{}, batchSize int, handler func([]interface{}) error) error {
	rows, err := r.ExecuteQuery(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	batch := make([]interface{}, 0, batchSize)

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return err
		}

		batch = append(batch, values)

		if len(batch) >= batchSize {
			if err := handler(batch); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}

	// Handle remaining items
	if len(batch) > 0 {
		if err := handler(batch); err != nil {
			return err
		}
	}

	return rows.Err()
}

// InvalidateCache removes cached entries
func (r *BaseRepository) InvalidateCache(ctx context.Context, pattern string) error {
	if r.cache == nil {
		return nil
	}

	return r.cache.Clear(ctx, pattern)
}

// HealthCheck performs a database health check
func (r *BaseRepository) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn := r.pool.GetPrimary()

	var result int
	err := conn.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("unexpected health check result: %d", result)
	}

	return nil
}
