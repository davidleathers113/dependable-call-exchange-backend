package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Monitor provides advanced database monitoring and optimization
type Monitor struct {
	pool   *ConnectionPool
	logger *zap.Logger
	config *MonitorConfig
}

// MonitorConfig holds monitoring configuration
type MonitorConfig struct {
	SlowQueryThresholdMs   int64
	LockWaitThresholdMs    int64
	IndexBloatThreshold    float64
	TableBloatThreshold    float64
	ReplicationLagThreshold time.Duration
	ConnectionThreshold    int
}

// QueryStats represents query performance statistics
type QueryStats struct {
	QueryID          string
	Query            string
	Calls            int64
	TotalTimeMs      float64
	MinTimeMs        float64
	MaxTimeMs        float64
	MeanTimeMs       float64
	StddevTimeMs     float64
	Rows             int64
	SharedBlksHit    int64
	SharedBlksRead   int64
	SharedBlksDirtied int64
	SharedBlksWritten int64
	TempBlksRead     int64
	TempBlksWritten  int64
}

// TableStats represents table statistics
type TableStats struct {
	SchemaName       string
	TableName        string
	TableSize        int64
	IndexSize        int64
	ToastSize        int64
	TotalSize        int64
	RowEstimate      int64
	DeadTuples       int64
	LiveTuples       int64
	BloatBytes       int64
	BloatPercentage  float64
	LastVacuum       *time.Time
	LastAutovacuum   *time.Time
	LastAnalyze      *time.Time
	LastAutoanalyze  *time.Time
}

// IndexStats represents index statistics
type IndexStats struct {
	SchemaName      string
	TableName       string
	IndexName       string
	IndexSize       int64
	IndexScans      int64
	IndexTupRead    int64
	IndexTupFetch   int64
	BloatBytes      int64
	BloatPercentage float64
	IsUnused        bool
	IsDuplicate     bool
	IsInvalid       bool
}

// ConnectionStats represents connection statistics
type ConnectionStats struct {
	TotalConnections   int
	ActiveConnections  int
	IdleConnections    int
	IdleInTransaction  int
	WaitingConnections int
	MaxConnections     int
	ConnectionsByState map[string]int
	ConnectionsByApp   map[string]int
}

// NewMonitor creates a new database monitor
func NewMonitor(pool *ConnectionPool, logger *zap.Logger, config *MonitorConfig) *Monitor {
	if config == nil {
		config = &MonitorConfig{
			SlowQueryThresholdMs:    1000,
			LockWaitThresholdMs:     5000,
			IndexBloatThreshold:     30.0,
			TableBloatThreshold:     30.0,
			ReplicationLagThreshold: 30 * time.Second,
			ConnectionThreshold:     80,
		}
	}

	return &Monitor{
		pool:   pool,
		logger: logger,
		config: config,
	}
}

// GetSlowQueries retrieves slow queries from pg_stat_statements
func (m *Monitor) GetSlowQueries(ctx context.Context, limit int) ([]QueryStats, error) {
	query := `
		SELECT 
			queryid::text,
			query,
			calls,
			total_exec_time,
			min_exec_time,
			max_exec_time,
			mean_exec_time,
			stddev_exec_time,
			rows,
			shared_blks_hit,
			shared_blks_read,
			shared_blks_dirtied,
			shared_blks_written,
			temp_blks_read,
			temp_blks_written
		FROM pg_stat_statements
		WHERE mean_exec_time > $1
		ORDER BY mean_exec_time DESC
		LIMIT $2
	`

	rows, err := m.pool.GetPrimary().Query(ctx, query, m.config.SlowQueryThresholdMs, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get slow queries: %w", err)
	}
	defer rows.Close()

	var stats []QueryStats
	for rows.Next() {
		var s QueryStats
		err := rows.Scan(
			&s.QueryID,
			&s.Query,
			&s.Calls,
			&s.TotalTimeMs,
			&s.MinTimeMs,
			&s.MaxTimeMs,
			&s.MeanTimeMs,
			&s.StddevTimeMs,
			&s.Rows,
			&s.SharedBlksHit,
			&s.SharedBlksRead,
			&s.SharedBlksDirtied,
			&s.SharedBlksWritten,
			&s.TempBlksRead,
			&s.TempBlksWritten,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan query stats: %w", err)
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

// GetTableStats retrieves table statistics including bloat
func (m *Monitor) GetTableStats(ctx context.Context) ([]TableStats, error) {
	query := `
		WITH table_stats AS (
			SELECT
				schemaname,
				tablename,
				pg_table_size(schemaname||'.'||tablename) AS table_size,
				pg_indexes_size(schemaname||'.'||tablename) AS index_size,
				pg_total_relation_size(schemaname||'.'||tablename) AS total_size,
				n_live_tup,
				n_dead_tup,
				last_vacuum,
				last_autovacuum,
				last_analyze,
				last_autoanalyze
			FROM pg_stat_user_tables
		),
		bloat_stats AS (
			SELECT
				schemaname,
				tablename,
				pg_size_pretty(raw_waste) as waste,
				raw_waste,
				round(raw_waste::numeric / pg_table_size(schemaname||'.'||tablename) * 100, 2) AS bloat_pct
			FROM (
				SELECT
					schemaname,
					tablename,
					(pg_stat_get_live_tuples(c.oid) + pg_stat_get_dead_tuples(c.oid)) * 
					current_setting('block_size')::integer - pg_table_size(schemaname||'.'||tablename) AS raw_waste
				FROM pg_stat_user_tables
				JOIN pg_class c ON c.relname = tablename AND c.relnamespace = 
					(SELECT oid FROM pg_namespace WHERE nspname = schemaname)
			) bloat_calc
			WHERE raw_waste > 0
		)
		SELECT
			ts.schemaname,
			ts.tablename,
			ts.table_size,
			ts.index_size,
			ts.total_size - ts.table_size - ts.index_size AS toast_size,
			ts.total_size,
			ts.n_live_tup,
			ts.n_dead_tup,
			COALESCE(bs.raw_waste, 0) AS bloat_bytes,
			COALESCE(bs.bloat_pct, 0) AS bloat_percentage,
			ts.last_vacuum,
			ts.last_autovacuum,
			ts.last_analyze,
			ts.last_autoanalyze
		FROM table_stats ts
		LEFT JOIN bloat_stats bs USING (schemaname, tablename)
		ORDER BY ts.total_size DESC
	`

	rows, err := m.pool.GetPrimary().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get table stats: %w", err)
	}
	defer rows.Close()

	var stats []TableStats
	for rows.Next() {
		var s TableStats
		err := rows.Scan(
			&s.SchemaName,
			&s.TableName,
			&s.TableSize,
			&s.IndexSize,
			&s.ToastSize,
			&s.TotalSize,
			&s.LiveTuples,
			&s.DeadTuples,
			&s.BloatBytes,
			&s.BloatPercentage,
			&s.LastVacuum,
			&s.LastAutovacuum,
			&s.LastAnalyze,
			&s.LastAutoanalyze,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan table stats: %w", err)
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

// GetIndexStats retrieves index statistics including unused and duplicate indexes
func (m *Monitor) GetIndexStats(ctx context.Context) ([]IndexStats, error) {
	query := `
		WITH index_stats AS (
			SELECT
				schemaname,
				tablename,
				indexname,
				pg_relation_size(indexrelid) AS index_size,
				idx_scan,
				idx_tup_read,
				idx_tup_fetch
			FROM pg_stat_user_indexes
		),
		duplicate_indexes AS (
			SELECT 
				indrelid::regclass::text AS tablename,
				array_agg(indexrelid::regclass::text) AS duplicate_indexes
			FROM pg_index
			GROUP BY indrelid, indkey, indexprs, indpred
			HAVING count(*) > 1
		),
		invalid_indexes AS (
			SELECT 
				n.nspname AS schemaname,
				c.relname AS indexname
			FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_index i ON i.indexrelid = c.oid
			WHERE i.indisvalid = false
		)
		SELECT
			is.schemaname,
			is.tablename,
			is.indexname,
			is.index_size,
			is.idx_scan,
			is.idx_tup_read,
			is.idx_tup_fetch,
			0 AS bloat_bytes, -- Calculate separately if needed
			0.0 AS bloat_percentage,
			CASE WHEN is.idx_scan = 0 THEN true ELSE false END AS is_unused,
			CASE WHEN di.duplicate_indexes IS NOT NULL THEN true ELSE false END AS is_duplicate,
			CASE WHEN ii.indexname IS NOT NULL THEN true ELSE false END AS is_invalid
		FROM index_stats is
		LEFT JOIN duplicate_indexes di ON is.tablename = di.tablename 
			AND is.indexname = ANY(di.duplicate_indexes)
		LEFT JOIN invalid_indexes ii ON is.schemaname = ii.schemaname 
			AND is.indexname = ii.indexname
		ORDER BY is.index_size DESC
	`

	rows, err := m.pool.GetPrimary().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get index stats: %w", err)
	}
	defer rows.Close()

	var stats []IndexStats
	for rows.Next() {
		var s IndexStats
		err := rows.Scan(
			&s.SchemaName,
			&s.TableName,
			&s.IndexName,
			&s.IndexSize,
			&s.IndexScans,
			&s.IndexTupRead,
			&s.IndexTupFetch,
			&s.BloatBytes,
			&s.BloatPercentage,
			&s.IsUnused,
			&s.IsDuplicate,
			&s.IsInvalid,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan index stats: %w", err)
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

// GetConnectionStats retrieves connection statistics
func (m *Monitor) GetConnectionStats(ctx context.Context) (*ConnectionStats, error) {
	query := `
		WITH connection_stats AS (
			SELECT
				state,
				application_name,
				count(*) AS count
			FROM pg_stat_activity
			WHERE pid != pg_backend_pid()
			GROUP BY GROUPING SETS ((state), (application_name), ())
		)
		SELECT
			COALESCE(state, 'total') AS category,
			COALESCE(application_name, '') AS app_name,
			count
		FROM connection_stats
		ORDER BY category, app_name
	`

	rows, err := m.pool.GetPrimary().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection stats: %w", err)
	}
	defer rows.Close()

	stats := &ConnectionStats{
		ConnectionsByState: make(map[string]int),
		ConnectionsByApp:   make(map[string]int),
	}

	for rows.Next() {
		var category, appName string
		var count int
		
		err := rows.Scan(&category, &appName, &count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan connection stats: %w", err)
		}

		switch category {
		case "total":
			stats.TotalConnections = count
		case "active":
			stats.ActiveConnections = count
		case "idle":
			stats.IdleConnections = count
		case "idle in transaction":
			stats.IdleInTransaction = count
		default:
			if appName != "" {
				stats.ConnectionsByApp[appName] = count
			} else {
				stats.ConnectionsByState[category] = count
			}
		}
	}

	// Get max connections
	var maxConnections int
	err = m.pool.GetPrimary().QueryRow(ctx, "SHOW max_connections").Scan(&maxConnections)
	if err == nil {
		stats.MaxConnections = maxConnections
	}

	return stats, rows.Err()
}

// GetLockingQueries retrieves queries that are holding locks
func (m *Monitor) GetLockingQueries(ctx context.Context) ([]map[string]interface{}, error) {
	query := `
		SELECT
			blocked_locks.pid AS blocked_pid,
			blocked_activity.usename AS blocked_user,
			blocking_locks.pid AS blocking_pid,
			blocking_activity.usename AS blocking_user,
			blocked_activity.query AS blocked_query,
			blocking_activity.query AS blocking_query,
			blocked_activity.application_name AS blocked_app,
			blocking_activity.application_name AS blocking_app,
			EXTRACT(EPOCH FROM (NOW() - blocked_activity.query_start))::INTEGER AS blocked_duration,
			EXTRACT(EPOCH FROM (NOW() - blocking_activity.query_start))::INTEGER AS blocking_duration
		FROM pg_locks blocked_locks
		JOIN pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
		JOIN pg_locks blocking_locks ON blocking_locks.locktype = blocked_locks.locktype
			AND blocking_locks.DATABASE IS NOT DISTINCT FROM blocked_locks.DATABASE
			AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
			AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
			AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
			AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
			AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
			AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
			AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
			AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
			AND blocking_locks.pid != blocked_locks.pid
		JOIN pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
		WHERE NOT blocked_locks.GRANTED
		AND blocking_locks.GRANTED
	`

	rows, err := m.pool.GetPrimary().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get locking queries: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var blockedPid, blockingPid, blockedDuration, blockingDuration int
		var blockedUser, blockingUser, blockedQuery, blockingQuery, blockedApp, blockingApp string

		err := rows.Scan(
			&blockedPid, &blockedUser,
			&blockingPid, &blockingUser,
			&blockedQuery, &blockingQuery,
			&blockedApp, &blockingApp,
			&blockedDuration, &blockingDuration,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan locking queries: %w", err)
		}

		results = append(results, map[string]interface{}{
			"blocked_pid":       blockedPid,
			"blocked_user":      blockedUser,
			"blocking_pid":      blockingPid,
			"blocking_user":     blockingUser,
			"blocked_query":     blockedQuery,
			"blocking_query":    blockingQuery,
			"blocked_app":       blockedApp,
			"blocking_app":      blockingApp,
			"blocked_duration":  blockedDuration,
			"blocking_duration": blockingDuration,
		})
	}

	return results, rows.Err()
}

// SuggestMissingIndexes analyzes queries and suggests missing indexes
func (m *Monitor) SuggestMissingIndexes(ctx context.Context) ([]string, error) {
	query := `
		WITH index_suggestions AS (
			SELECT
				schemaname,
				tablename,
				attname,
				n_distinct,
				avg_width,
				correlation
			FROM pg_stats
			WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
				AND n_distinct > 100
				AND correlation < 0.1
				AND NOT EXISTS (
					SELECT 1
					FROM pg_index i
					JOIN pg_class c ON c.oid = i.indrelid
					JOIN pg_namespace n ON n.oid = c.relnamespace
					JOIN pg_attribute a ON a.attrelid = c.oid
					WHERE n.nspname = schemaname
						AND c.relname = tablename
						AND a.attname = attname
						AND a.attnum = ANY(i.indkey)
				)
		)
		SELECT
			format('CREATE INDEX idx_%s_%s ON %I.%I (%I);',
				tablename,
				attname,
				schemaname,
				tablename,
				attname
			) AS index_suggestion
		FROM index_suggestions
		ORDER BY n_distinct DESC
		LIMIT 20
	`

	rows, err := m.pool.GetPrimary().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to suggest indexes: %w", err)
	}
	defer rows.Close()

	var suggestions []string
	for rows.Next() {
		var suggestion string
		if err := rows.Scan(&suggestion); err != nil {
			return nil, fmt.Errorf("failed to scan index suggestion: %w", err)
		}
		suggestions = append(suggestions, suggestion)
	}

	return suggestions, rows.Err()
}

// RunHealthCheck performs comprehensive health check
func (m *Monitor) RunHealthCheck(ctx context.Context) (map[string]interface{}, error) {
	results := make(map[string]interface{})

	// Check basic connectivity
	var pingResult int
	err := m.pool.GetPrimary().QueryRow(ctx, "SELECT 1").Scan(&pingResult)
	results["ping"] = err == nil

	// Check replication lag
	var lagSeconds sql.NullInt64
	err = m.pool.GetPrimary().QueryRow(ctx, `
		SELECT EXTRACT(EPOCH FROM (NOW() - pg_last_xact_replay_timestamp()))::INTEGER
	`).Scan(&lagSeconds)
	if err == nil && lagSeconds.Valid {
		lag := time.Duration(lagSeconds.Int64) * time.Second
		results["replication_lag"] = lag.String()
		results["replication_healthy"] = lag < m.config.ReplicationLagThreshold
	}

	// Check connection saturation
	connStats, err := m.GetConnectionStats(ctx)
	if err == nil {
		saturation := float64(connStats.TotalConnections) / float64(connStats.MaxConnections) * 100
		results["connection_saturation"] = saturation
		results["connection_healthy"] = saturation < float64(m.config.ConnectionThreshold)
	}

	// Check for long-running queries
	var longRunningCount int
	err = m.pool.GetPrimary().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM pg_stat_activity
		WHERE state != 'idle'
			AND query_start < NOW() - INTERVAL '5 minutes'
			AND pid != pg_backend_pid()
	`).Scan(&longRunningCount)
	if err == nil {
		results["long_running_queries"] = longRunningCount
		results["queries_healthy"] = longRunningCount == 0
	}

	// Check table bloat
	tableStats, err := m.GetTableStats(ctx)
	if err == nil {
		bloatedTables := 0
		for _, table := range tableStats {
			if table.BloatPercentage > m.config.TableBloatThreshold {
				bloatedTables++
			}
		}
		results["bloated_tables"] = bloatedTables
		results["bloat_healthy"] = bloatedTables == 0
	}

	// Overall health
	overallHealthy := true
	for key, value := range results {
		if strings.HasSuffix(key, "_healthy") {
			if healthy, ok := value.(bool); ok && !healthy {
				overallHealthy = false
				break
			}
		}
	}
	results["overall_healthy"] = overallHealthy

	return results, nil
}

// GeneratePerformanceReport generates a comprehensive performance report
func (m *Monitor) GeneratePerformanceReport(ctx context.Context) (string, error) {
	var report strings.Builder

	report.WriteString("=== DATABASE PERFORMANCE REPORT ===\n")
	report.WriteString(fmt.Sprintf("Generated at: %s\n\n", time.Now().Format(time.RFC3339)))

	// Health check summary
	health, err := m.RunHealthCheck(ctx)
	if err == nil {
		report.WriteString("## HEALTH CHECK ##\n")
		for key, value := range health {
			report.WriteString(fmt.Sprintf("- %s: %v\n", key, value))
		}
		report.WriteString("\n")
	}

	// Slow queries
	slowQueries, err := m.GetSlowQueries(ctx, 10)
	if err == nil {
		report.WriteString("## TOP 10 SLOW QUERIES ##\n")
		for i, query := range slowQueries {
			report.WriteString(fmt.Sprintf("%d. Mean time: %.2fms, Calls: %d\n", 
				i+1, query.MeanTimeMs, query.Calls))
			report.WriteString(fmt.Sprintf("   Query: %s\n\n", 
				strings.ReplaceAll(query.Query, "\n", " ")))
		}
	}

	// Connection stats
	connStats, err := m.GetConnectionStats(ctx)
	if err == nil {
		report.WriteString("## CONNECTION STATISTICS ##\n")
		report.WriteString(fmt.Sprintf("- Total connections: %d/%d\n", 
			connStats.TotalConnections, connStats.MaxConnections))
		report.WriteString(fmt.Sprintf("- Active: %d, Idle: %d, Idle in transaction: %d\n",
			connStats.ActiveConnections, connStats.IdleConnections, connStats.IdleInTransaction))
		report.WriteString("\n")
	}

	// Missing indexes
	missingIndexes, err := m.SuggestMissingIndexes(ctx)
	if err == nil && len(missingIndexes) > 0 {
		report.WriteString("## SUGGESTED INDEXES ##\n")
		for _, index := range missingIndexes {
			report.WriteString(fmt.Sprintf("- %s\n", index))
		}
		report.WriteString("\n")
	}

	return report.String(), nil
}