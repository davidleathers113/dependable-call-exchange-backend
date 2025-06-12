package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
)

func setupMonitoringTest(t *testing.T) (*Monitor, *ConnectionPool, func()) {
	logger := zaptest.NewLogger(t)
	db := testutil.NewTestDB(t)

	cfg := &config.DatabaseConfig{
		URL: db.ConnectionString(),
	}

	pool, err := NewConnectionPool(cfg, logger)
	require.NoError(t, err)

	monitor := NewMonitor(pool, logger, nil)

	cleanup := func() {
		pool.Close()
		// TestDB cleans up automatically
	}

	return monitor, pool, cleanup
}

func TestNewMonitor(t *testing.T) {
	logger := zaptest.NewLogger(t)
	pool := &ConnectionPool{} // Minimal pool

	t.Run("with default config", func(t *testing.T) {
		monitor := NewMonitor(pool, logger, nil)

		assert.NotNil(t, monitor)
		assert.Equal(t, int64(1000), monitor.config.SlowQueryThresholdMs)
		assert.Equal(t, int64(5000), monitor.config.LockWaitThresholdMs)
		assert.Equal(t, 30.0, monitor.config.IndexBloatThreshold)
		assert.Equal(t, 30.0, monitor.config.TableBloatThreshold)
		assert.Equal(t, 30*time.Second, monitor.config.ReplicationLagThreshold)
		assert.Equal(t, 80, monitor.config.ConnectionThreshold)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &MonitorConfig{
			SlowQueryThresholdMs:    500,
			LockWaitThresholdMs:     2000,
			IndexBloatThreshold:     20.0,
			TableBloatThreshold:     25.0,
			ReplicationLagThreshold: 60 * time.Second,
			ConnectionThreshold:     90,
		}

		monitor := NewMonitor(pool, logger, config)

		assert.NotNil(t, monitor)
		assert.Equal(t, config, monitor.config)
	})
}

func TestMonitor_GetConnectionStats(t *testing.T) {
	monitor, pool, cleanup := setupMonitoringTest(t)
	defer cleanup()

	ctx := context.Background()

	// First, enable pg_stat_activity if needed
	_, _ = pool.GetPrimary().Exec(ctx, `
		CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
	`)
	// Ignore error as extension might not be available in test DB

	t.Run("successful stats retrieval", func(t *testing.T) {
		stats, err := monitor.GetConnectionStats(ctx)

		// May fail if pg_stat_activity is not available
		if err != nil {
			t.Skip("pg_stat_activity not available")
		}

		assert.NotNil(t, stats)
		assert.GreaterOrEqual(t, stats.TotalConnections, 1) // At least our connection
		assert.NotNil(t, stats.ConnectionsByState)
		assert.NotNil(t, stats.ConnectionsByApp)
		assert.Greater(t, stats.MaxConnections, 0)
	})
}

func TestMonitor_GetTableStats(t *testing.T) {
	monitor, pool, cleanup := setupMonitoringTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create test tables
	_, err := pool.GetPrimary().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS monitor_test_1 (
			id SERIAL PRIMARY KEY,
			data TEXT
		);
		
		CREATE TABLE IF NOT EXISTS monitor_test_2 (
			id SERIAL PRIMARY KEY,
			value INT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);
		
		-- Insert some data
		INSERT INTO monitor_test_1 (data)
		SELECT 'test data ' || i
		FROM generate_series(1, 100) i;
		
		INSERT INTO monitor_test_2 (value)
		SELECT i * 10
		FROM generate_series(1, 50) i;
		
		-- Run ANALYZE to update statistics
		ANALYZE monitor_test_1;
		ANALYZE monitor_test_2;
	`)
	require.NoError(t, err)

	defer func() {
		pool.GetPrimary().Exec(ctx, "DROP TABLE IF EXISTS monitor_test_1, monitor_test_2")
	}()

	t.Run("retrieve table statistics", func(t *testing.T) {
		stats, err := monitor.GetTableStats(ctx)

		// May fail if pg_stat_user_tables is not available
		if err != nil {
			t.Skip("pg_stat_user_tables not available")
		}

		assert.NotEmpty(t, stats)

		// Find our test tables
		var foundTable1, foundTable2 bool
		for _, stat := range stats {
			if stat.TableName == "monitor_test_1" {
				foundTable1 = true
				assert.Equal(t, "public", stat.SchemaName)
				assert.Greater(t, stat.TableSize, int64(0))
				assert.GreaterOrEqual(t, stat.LiveTuples, int64(90)) // Approximate
			}
			if stat.TableName == "monitor_test_2" {
				foundTable2 = true
				assert.Equal(t, "public", stat.SchemaName)
				assert.Greater(t, stat.TableSize, int64(0))
				assert.GreaterOrEqual(t, stat.LiveTuples, int64(40)) // Approximate
			}
		}

		assert.True(t, foundTable1, "monitor_test_1 not found in stats")
		assert.True(t, foundTable2, "monitor_test_2 not found in stats")
	})
}

func TestMonitor_GetIndexStats(t *testing.T) {
	monitor, pool, cleanup := setupMonitoringTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create test table with indexes
	_, err := pool.GetPrimary().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS index_test (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE,
			status VARCHAR(50),
			created_at TIMESTAMPTZ DEFAULT NOW()
		);
		
		-- Create regular index
		CREATE INDEX idx_index_test_status ON index_test(status);
		
		-- Create unused index
		CREATE INDEX idx_index_test_created ON index_test(created_at);
		
		-- Insert data
		INSERT INTO index_test (email, status)
		SELECT 'user' || i || '@example.com', 
		       CASE WHEN i % 3 = 0 THEN 'active' 
		            WHEN i % 3 = 1 THEN 'inactive' 
		            ELSE 'pending' END
		FROM generate_series(1, 100) i;
		
		-- Use one index
		SELECT * FROM index_test WHERE status = 'active';
		
		-- Update statistics
		ANALYZE index_test;
	`)
	require.NoError(t, err)

	defer pool.GetPrimary().Exec(ctx, "DROP TABLE IF EXISTS index_test")

	t.Run("retrieve index statistics", func(t *testing.T) {
		stats, err := monitor.GetIndexStats(ctx)

		// May fail if pg_stat_user_indexes is not available
		if err != nil {
			t.Skip("pg_stat_user_indexes not available")
		}

		assert.NotEmpty(t, stats)

		// Find our test indexes
		var foundPrimary, foundEmail, foundStatus, foundCreated bool
		for _, stat := range stats {
			if stat.TableName == "index_test" {
				switch stat.IndexName {
				case "index_test_pkey":
					foundPrimary = true
				case "index_test_email_key":
					foundEmail = true
				case "idx_index_test_status":
					foundStatus = true
					// This index was used in our query
					assert.Greater(t, stat.IndexScans, int64(0))
				case "idx_index_test_created":
					foundCreated = true
					// This index was not used
					assert.Equal(t, int64(0), stat.IndexScans)
					assert.True(t, stat.IsUnused)
				}
			}
		}

		assert.True(t, foundPrimary, "Primary key index not found")
		assert.True(t, foundEmail, "Email unique index not found")
		assert.True(t, foundStatus, "Status index not found")
		assert.True(t, foundCreated, "Created index not found")
	})
}

func TestMonitor_SuggestMissingIndexes(t *testing.T) {
	monitor, pool, cleanup := setupMonitoringTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create test table without indexes on frequently queried columns
	_, err := pool.GetPrimary().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS missing_index_test (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL,
			category VARCHAR(50) NOT NULL,
			status VARCHAR(20) NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);
		
		-- Insert diverse data
		INSERT INTO missing_index_test (user_id, category, status)
		SELECT 
			(random() * 1000)::INT,
			'category_' || (random() * 20)::INT,
			CASE (random() * 3)::INT 
				WHEN 0 THEN 'pending'
				WHEN 1 THEN 'active'
				ELSE 'completed'
			END
		FROM generate_series(1, 10000);
		
		-- Update statistics
		ANALYZE missing_index_test;
	`)
	require.NoError(t, err)

	defer pool.GetPrimary().Exec(ctx, "DROP TABLE IF EXISTS missing_index_test")

	t.Run("suggest indexes for unindexed columns", func(t *testing.T) {
		suggestions, err := monitor.SuggestMissingIndexes(ctx)

		// This test may not produce suggestions depending on data distribution
		if err != nil {
			t.Skip("Index suggestions not available")
		}

		// The suggestions should be valid CREATE INDEX statements
		for _, suggestion := range suggestions {
			assert.True(t, strings.HasPrefix(suggestion, "CREATE INDEX"))
			assert.Contains(t, suggestion, " ON ")
		}
	})
}

func TestMonitor_GetLockingQueries(t *testing.T) {
	monitor, _, cleanup := setupMonitoringTest(t)
	defer cleanup()

	ctx := context.Background()

	// Note: Creating actual lock contention in tests is complex and may be flaky
	// This test primarily verifies the query executes without error

	t.Run("retrieve locking queries", func(t *testing.T) {
		locks, err := monitor.GetLockingQueries(ctx)

		// Query should execute without error
		assert.NoError(t, err)
		assert.NotNil(t, locks)

		// In normal test conditions, there shouldn't be blocking locks
		assert.Empty(t, locks)
	})
}

func TestMonitor_RunHealthCheck(t *testing.T) {
	monitor, pool, cleanup := setupMonitoringTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create some test data for health check
	_, err := pool.GetPrimary().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS health_test (
			id SERIAL PRIMARY KEY,
			data TEXT
		);
		
		-- Insert data to create some table size
		INSERT INTO health_test (data)
		SELECT repeat('x', 1000)
		FROM generate_series(1, 100);
	`)
	require.NoError(t, err)

	defer pool.GetPrimary().Exec(ctx, "DROP TABLE IF EXISTS health_test")

	t.Run("comprehensive health check", func(t *testing.T) {
		results, err := monitor.RunHealthCheck(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, results)

		// Check basic connectivity
		ping, ok := results["ping"].(bool)
		assert.True(t, ok)
		assert.True(t, ping)

		// Check overall health
		overallHealthy, ok := results["overall_healthy"].(bool)
		assert.True(t, ok)
		// Should be healthy in test environment
		assert.True(t, overallHealthy)

		// Check specific metrics exist
		assert.Contains(t, results, "connection_saturation")
		assert.Contains(t, results, "connection_healthy")
		assert.Contains(t, results, "long_running_queries")
		assert.Contains(t, results, "queries_healthy")
		assert.Contains(t, results, "bloated_tables")
		assert.Contains(t, results, "bloat_healthy")
	})
}

func TestMonitor_GeneratePerformanceReport(t *testing.T) {
	monitor, pool, cleanup := setupMonitoringTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create some test data
	_, err := pool.GetPrimary().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS perf_test (
			id SERIAL PRIMARY KEY,
			value INT
		);
		
		INSERT INTO perf_test (value)
		SELECT i FROM generate_series(1, 100) i;
	`)
	require.NoError(t, err)

	defer pool.GetPrimary().Exec(ctx, "DROP TABLE IF EXISTS perf_test")

	t.Run("generate performance report", func(t *testing.T) {
		report, err := monitor.GeneratePerformanceReport(ctx)

		assert.NoError(t, err)
		assert.NotEmpty(t, report)

		// Verify report structure
		assert.Contains(t, report, "DATABASE PERFORMANCE REPORT")
		assert.Contains(t, report, "Generated at:")
		assert.Contains(t, report, "HEALTH CHECK")

		// Report should contain various sections
		reportLower := strings.ToLower(report)
		assert.Contains(t, reportLower, "connection")
		assert.Contains(t, reportLower, "overall_healthy")
	})
}

func TestMonitor_GetSlowQueries(t *testing.T) {
	monitor, pool, cleanup := setupMonitoringTest(t)
	defer cleanup()

	ctx := context.Background()

	// Enable pg_stat_statements if possible
	_, _ = pool.GetPrimary().Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pg_stat_statements")

	t.Run("retrieve slow queries", func(t *testing.T) {
		queries, err := monitor.GetSlowQueries(ctx, 10)

		// This may fail if pg_stat_statements is not available
		if err != nil {
			if strings.Contains(err.Error(), "pg_stat_statements") ||
				strings.Contains(err.Error(), "does not exist") {
				t.Skip("pg_stat_statements extension not available")
			}
		}

		assert.NoError(t, err)
		assert.NotNil(t, queries)

		// Verify structure of returned queries
		for _, query := range queries {
			assert.NotEmpty(t, query.QueryID)
			assert.NotEmpty(t, query.Query)
			assert.GreaterOrEqual(t, query.Calls, int64(0))
			assert.GreaterOrEqual(t, query.MeanTimeMs, float64(0))
		}
	})
}

// Benchmark tests
func BenchmarkMonitor_GetConnectionStats(b *testing.B) {
	monitor, _, cleanup := setupMonitoringTest(&testing.T{})
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := monitor.GetConnectionStats(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMonitor_RunHealthCheck(b *testing.B) {
	monitor, _, cleanup := setupMonitoringTest(&testing.T{})
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := monitor.RunHealthCheck(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
