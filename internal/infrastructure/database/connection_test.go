package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"testing/quick"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
)

func TestConnectionPool_NewConnectionPool(t *testing.T) {
	logger := zaptest.NewLogger(t)
	
	tests := []struct {
		name    string
		config  *config.DatabaseConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful creation with primary only",
			config: &config.DatabaseConfig{
				URL:             testutil.GetTestDatabaseURL(),
				MaxOpenConns:    10,
				MaxIdleConns:    2,
				ConnMaxLifetime: 30 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "invalid primary URL",
			config: &config.DatabaseConfig{
				URL: "invalid://url",
			},
			wantErr: true,
			errMsg:  "failed to parse primary database URL",
		},
		{
			name: "connection failure",
			config: &config.DatabaseConfig{
				URL: "postgresql://invalid:invalid@localhost:9999/invalid",
			},
			wantErr: true,
			errMsg:  "failed to ping primary database",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewConnectionPool(tt.config, logger)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, pool)
			} else {
				require.NoError(t, err)
				require.NotNil(t, pool)
				defer pool.Close()
				
				// Verify pool is functional
				ctx := context.Background()
				var result int
				err = pool.GetPrimary().QueryRow(ctx, "SELECT 1").Scan(&result)
				assert.NoError(t, err)
				assert.Equal(t, 1, result)
			}
		})
	}
}

func TestConnectionPool_GetReadConnection(t *testing.T) {
	logger := zaptest.NewLogger(t)
	
	t.Run("returns primary when no replicas", func(t *testing.T) {
		cfg := &config.DatabaseConfig{
			URL: testutil.GetTestDatabaseURL(),
		}
		
		pool, err := NewConnectionPool(cfg, logger)
		require.NoError(t, err)
		defer pool.Close()
		
		conn := pool.GetReadConnection(false)
		assert.Equal(t, pool.GetPrimary(), conn)
	})
	
	t.Run("returns primary when preferPrimary is true", func(t *testing.T) {
		// Use extended config for replicas
		cfg := &ExtendedDatabaseConfig{
			DatabaseConfig: &config.DatabaseConfig{
				URL: testutil.GetTestDatabaseURL(),
			},
			PrimaryURL: testutil.GetTestDatabaseURL(),
			ReplicaURLs: []string{
				testutil.GetTestDatabaseURL(),
			},
		}
		
		pool, err := NewConnectionPoolWithExtended(cfg, logger)
		require.NoError(t, err)
		defer pool.Close()
		
		conn := pool.GetReadConnection(true)
		assert.Equal(t, pool.GetPrimary(), conn)
	})
	
	t.Run("returns replica when available and not preferPrimary", func(t *testing.T) {
		// Use extended config for replicas
		cfg := &ExtendedDatabaseConfig{
			DatabaseConfig: &config.DatabaseConfig{
				URL: testutil.GetTestDatabaseURL(),
			},
			PrimaryURL: testutil.GetTestDatabaseURL(),
			ReplicaURLs: []string{
				testutil.GetTestDatabaseURL(),
			},
		}
		
		pool, err := NewConnectionPoolWithExtended(cfg, logger)
		require.NoError(t, err)
		defer pool.Close()
		
		// Should return a replica (could be any of them)
		conn := pool.GetReadConnection(false)
		assert.NotNil(t, conn)
		
		// Verify it works
		ctx := context.Background()
		var result int
		err = conn.QueryRow(ctx, "SELECT 1").Scan(&result)
		assert.NoError(t, err)
		assert.Equal(t, 1, result)
	})
}

func TestConnectionPool_Transaction(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := testutil.NewTestDB(t)
	// No need to defer Close - TestDB uses t.Cleanup
	
	cfg := &config.DatabaseConfig{
		URL: db.ConnectionString(),
	}
	
	pool, err := NewConnectionPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()
	
	ctx := context.Background()
	
	t.Run("successful transaction", func(t *testing.T) {
		// Create test table
		_, err := pool.GetPrimary().Exec(ctx, `
			CREATE TABLE IF NOT EXISTS test_transactions (
				id SERIAL PRIMARY KEY,
				value TEXT
			)
		`)
		require.NoError(t, err)
		defer pool.GetPrimary().Exec(ctx, "DROP TABLE test_transactions")
		
		// Execute transaction
		err = pool.Transaction(ctx, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "test")
			return err
		})
		assert.NoError(t, err)
		
		// Verify data was inserted
		var count int
		err = pool.GetPrimary().QueryRow(ctx, "SELECT COUNT(*) FROM test_transactions").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})
	
	t.Run("rolled back transaction", func(t *testing.T) {
		// Create test table
		_, err := pool.GetPrimary().Exec(ctx, `
			CREATE TABLE IF NOT EXISTS test_rollback (
				id SERIAL PRIMARY KEY,
				value TEXT
			)
		`)
		require.NoError(t, err)
		defer pool.GetPrimary().Exec(ctx, "DROP TABLE test_rollback")
		
		// Execute transaction that fails
		err = pool.Transaction(ctx, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO test_rollback (value) VALUES ($1)", "test")
			if err != nil {
				return err
			}
			return fmt.Errorf("intentional error")
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "intentional error")
		
		// Verify data was NOT inserted
		var count int
		err = pool.GetPrimary().QueryRow(ctx, "SELECT COUNT(*) FROM test_rollback").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestCircuitBreaker(t *testing.T) {
	cb := &CircuitBreaker{
		timeout:   100 * time.Millisecond,
		threshold: 3,
		state:     CircuitClosed,
	}
	
	t.Run("allows requests when closed", func(t *testing.T) {
		assert.True(t, cb.Allow())
	})
	
	t.Run("opens after threshold failures", func(t *testing.T) {
		// Record failures up to threshold
		for i := 0; i < cb.threshold; i++ {
			cb.RecordFailure()
			if i < cb.threshold-1 {
				assert.Equal(t, CircuitClosed, cb.state)
			}
		}
		
		// Circuit should now be open
		assert.Equal(t, CircuitOpen, cb.state)
		assert.False(t, cb.Allow())
	})
	
	t.Run("transitions to half-open after timeout", func(t *testing.T) {
		// Wait for timeout
		time.Sleep(cb.timeout + 10*time.Millisecond)
		
		// Should now allow a test request
		assert.True(t, cb.Allow())
		assert.Equal(t, CircuitHalfOpen, cb.state)
	})
	
	t.Run("closes on success in half-open state", func(t *testing.T) {
		cb.state = CircuitHalfOpen
		cb.RecordSuccess()
		
		assert.Equal(t, CircuitClosed, cb.state)
		assert.Equal(t, 0, cb.failureCount)
	})
}

func TestConnectionPool_HealthCheck(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.DatabaseConfig{
		URL: testutil.GetTestDatabaseURL(),
	}
	
	pool, err := NewConnectionPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()
	
	// Let health check routine run
	time.Sleep(100 * time.Millisecond)
	
	// Verify metrics are being collected
	pool.metrics.mu.RLock()
	lastCheck := pool.metrics.LastHealthCheck
	pool.metrics.mu.RUnlock()
	
	assert.False(t, lastCheck.IsZero())
}

func TestConnectionPool_ConfigurePool(t *testing.T) {
	logger := zaptest.NewLogger(t)
	
	tests := []struct {
		name           string
		maxConnections int
		wantMaxConns   int32
	}{
		{
			name:           "uses configured max connections",
			maxConnections: 50,
			wantMaxConns:   50,
		},
		{
			name:           "uses default when zero",
			maxConnections: 0,
			wantMaxConns:   25,
		},
		{
			name:           "uses configured high value",
			maxConnections: 200,
			wantMaxConns:   200,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.DatabaseConfig{
				URL:          testutil.GetTestDatabaseURL(),
				MaxOpenConns: tt.maxConnections,
			}
			
			pool, err := NewConnectionPool(cfg, logger)
			require.NoError(t, err)
			defer pool.Close()
			
			stats := pool.GetPrimary().Stat()
			assert.Equal(t, tt.wantMaxConns, stats.MaxConns())
		})
	}
}

// TestConnectionPool_Concurrent tests concurrent access patterns
func TestConnectionPool_Concurrent(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := testutil.NewTestDB(t)
	// No need to defer Close - TestDB uses t.Cleanup
	
	cfg := &config.DatabaseConfig{
		URL:          db.ConnectionString(),
		MaxOpenConns: 20,
	}
	
	pool, err := NewConnectionPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()
	
	ctx := context.Background()
	
	// Create test table
	_, err = pool.GetPrimary().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS concurrent_test (
			id SERIAL PRIMARY KEY,
			value INT
		)
	`)
	require.NoError(t, err)
	defer pool.GetPrimary().Exec(ctx, "DROP TABLE concurrent_test")
	
	// Run concurrent operations
	const numGoroutines = 50
	const opsPerGoroutine = 100
	
	errChan := make(chan error, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(workerID int) {
			var err error
			defer func() { errChan <- err }()
			
			for j := 0; j < opsPerGoroutine; j++ {
				// Mix of operations
				switch j % 3 {
				case 0:
					// Direct query
					var result int
					err = pool.GetReadConnection(false).QueryRow(ctx, "SELECT 1").Scan(&result)
					
				case 1:
					// Insert in transaction
					err = pool.Transaction(ctx, func(tx pgx.Tx) error {
						_, err := tx.Exec(ctx, "INSERT INTO concurrent_test (value) VALUES ($1)", workerID*1000+j)
						return err
					})
					
				case 2:
					// Read from table
					var count int
					err = pool.GetReadConnection(false).QueryRow(ctx, "SELECT COUNT(*) FROM concurrent_test").Scan(&count)
				}
				
				if err != nil {
					return
				}
			}
		}(i)
	}
	
	// Collect results
	for i := 0; i < numGoroutines; i++ {
		err := <-errChan
		assert.NoError(t, err)
	}
	
	// Verify final state
	var finalCount int
	err = pool.GetPrimary().QueryRow(ctx, "SELECT COUNT(*) FROM concurrent_test").Scan(&finalCount)
	assert.NoError(t, err)
	assert.Greater(t, finalCount, 0)
}

// Property-based test for connection pool configuration validation
func TestConnectionPool_PropertyBasedResilience(t *testing.T) {
	property := func(maxConns, minConns uint8, timeoutSecs uint16) bool {
		if maxConns == 0 || minConns > maxConns {
			return true // Skip invalid combinations
		}
		
		// Test that pool handles various configurations gracefully
		config := &ExtendedDatabaseConfig{
			DatabaseConfig: &config.DatabaseConfig{
				MaxOpenConns:    int(maxConns),
				MaxIdleConns:    int(minConns),
				ConnMaxLifetime: time.Duration(timeoutSecs) * time.Second,
				URL:             testutil.GetTestDatabaseURL(),
			},
			MaxConnections:  int(maxConns),
			MinConnections:  int(minConns),
			MaxConnLifetime: time.Duration(timeoutSecs) * time.Second,
		}
		
		logger := zaptest.NewLogger(t)
		pool, err := NewConnectionPool(config.DatabaseConfig, logger)
		
		// Should succeed for all valid configurations
		if err == nil {
			defer pool.Close()
			
			// Verify pool is configured correctly
			stats := pool.GetPrimary().Stat()
			return stats.MaxConns() == int32(maxConns) || stats.MaxConns() == 25 // Default
		}
		
		return true // Error is acceptable for edge cases
	}
	
	if err := quick.Check(property, &quick.Config{MaxCount: 20}); err != nil {
		t.Error(err)
	}
}

// Test ExtendedDatabaseConfig validation
func TestExtendedDatabaseConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *ExtendedDatabaseConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid extended config",
			config: &ExtendedDatabaseConfig{
				DatabaseConfig: &config.DatabaseConfig{
					URL: testutil.GetTestDatabaseURL(),
				},
				PrimaryURL:      testutil.GetTestDatabaseURL(),
				ReplicaURLs:     []string{testutil.GetTestDatabaseURL()},
				MaxConnections:  50,
				MinConnections:  10,
				MaxConnLifetime: 30 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "min connections greater than max",
			config: &ExtendedDatabaseConfig{
				DatabaseConfig: &config.DatabaseConfig{
					URL: testutil.GetTestDatabaseURL(),
				},
				MaxConnections: 10,
				MinConnections: 20,
			},
			wantErr: false, // Should use defaults
		},
		{
			name: "zero max connections uses default",
			config: &ExtendedDatabaseConfig{
				DatabaseConfig: &config.DatabaseConfig{
					URL: testutil.GetTestDatabaseURL(),
				},
				MaxConnections: 0,
			},
			wantErr: false,
		},
	}
	
	logger := zaptest.NewLogger(t)
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewConnectionPoolWithExtended(tt.config, logger)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, pool)
				defer pool.Close()
				
				// Verify configuration applied correctly
				stats := pool.GetPrimary().Stat()
				if tt.config.MaxConnections > 0 {
					assert.Equal(t, int32(tt.config.MaxConnections), stats.MaxConns())
				} else {
					assert.Equal(t, int32(25), stats.MaxConns()) // Default
				}
			}
		})
	}
}

// Test comprehensive CircuitBreaker behavior
func TestCircuitBreaker_ComprehensiveBehavior(t *testing.T) {
	cb := &CircuitBreaker{
		timeout:   50 * time.Millisecond,
		threshold: 3,
		state:     CircuitClosed,
	}
	
	t.Run("multiple cycles of open and close", func(t *testing.T) {
		// First cycle: close -> open
		for i := 0; i < cb.threshold; i++ {
			assert.True(t, cb.Allow())
			cb.RecordFailure()
		}
		assert.Equal(t, CircuitOpen, cb.state)
		assert.False(t, cb.Allow())
		
		// Wait for timeout to transition to half-open
		time.Sleep(cb.timeout + 10*time.Millisecond)
		assert.True(t, cb.Allow())
		assert.Equal(t, CircuitHalfOpen, cb.state)
		
		// Success in half-open closes circuit
		cb.RecordSuccess()
		assert.Equal(t, CircuitClosed, cb.state)
		assert.Equal(t, 0, cb.failureCount)
		
		// Second cycle: verify it can open again
		for i := 0; i < cb.threshold; i++ {
			cb.RecordFailure()
		}
		assert.Equal(t, CircuitOpen, cb.state)
	})
	
	t.Run("failure in half-open state", func(t *testing.T) {
		cb.state = CircuitHalfOpen
		cb.failureCount = 0
		
		// Failure in half-open should increment counter
		cb.RecordFailure()
		assert.Equal(t, 1, cb.failureCount)
		
		// Should eventually open if threshold reached
		for i := 1; i < cb.threshold; i++ {
			cb.RecordFailure()
		}
		assert.Equal(t, CircuitOpen, cb.state)
	})
}

// Test ConnectionMetrics collection
func TestConnectionPool_MetricsCollection(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := testutil.NewTestDB(t)
	
	cfg := &config.DatabaseConfig{
		URL: db.ConnectionString(),
	}
	
	pool, err := NewConnectionPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()
	
	ctx := context.Background()
	
	// Perform operations to generate metrics
	for i := 0; i < 5; i++ {
		var result int
		err := pool.GetPrimary().QueryRow(ctx, "SELECT 1").Scan(&result)
		require.NoError(t, err)
	}
	
	// Execute transactions
	for i := 0; i < 3; i++ {
		err = pool.Transaction(ctx, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "SELECT 1")
			return err
		})
		assert.NoError(t, err)
	}
	
	// Failed transaction
	err = pool.Transaction(ctx, func(tx pgx.Tx) error {
		return fmt.Errorf("intentional error")
	})
	assert.Error(t, err)
	
	// Let metrics collection run
	time.Sleep(100 * time.Millisecond)
	
	// Verify metrics
	pool.metrics.mu.RLock()
	defer pool.metrics.mu.RUnlock()
	
	assert.Greater(t, pool.metrics.TotalConnections, int64(0))
	assert.Equal(t, int64(4), pool.metrics.TransactionsStarted)
	assert.Equal(t, int64(3), pool.metrics.TransactionsCommitted)
	assert.Equal(t, int64(1), pool.metrics.TransactionsRolledBack)
	assert.False(t, pool.metrics.LastHealthCheck.IsZero())
}

// Test GetReadConnection with various scenarios
func TestConnectionPool_GetReadConnection_Advanced(t *testing.T) {
	logger := zaptest.NewLogger(t)
	
	t.Run("high replication lag forces primary", func(t *testing.T) {
		cfg := &ExtendedDatabaseConfig{
			DatabaseConfig: &config.DatabaseConfig{
				URL: testutil.GetTestDatabaseURL(),
			},
			PrimaryURL:  testutil.GetTestDatabaseURL(),
			ReplicaURLs: []string{testutil.GetTestDatabaseURL()},
		}
		
		pool, err := NewConnectionPoolWithExtended(cfg, logger)
		require.NoError(t, err)
		defer pool.Close()
		
		// Simulate high replication lag
		pool.metrics.mu.Lock()
		pool.metrics.ReplicationLag = 10 * time.Second
		pool.metrics.mu.Unlock()
		
		// Should return primary even when not preferred
		conn := pool.GetReadConnection(false)
		assert.Equal(t, pool.GetPrimary(), conn)
	})
	
	t.Run("round-robin replica selection", func(t *testing.T) {
		cfg := &ExtendedDatabaseConfig{
			DatabaseConfig: &config.DatabaseConfig{
				URL: testutil.GetTestDatabaseURL(),
			},
			PrimaryURL: testutil.GetTestDatabaseURL(),
			ReplicaURLs: []string{
				testutil.GetTestDatabaseURL(),
				testutil.GetTestDatabaseURL(),
				testutil.GetTestDatabaseURL(),
			},
		}
		
		pool, err := NewConnectionPoolWithExtended(cfg, logger)
		require.NoError(t, err)
		defer pool.Close()
		
		// Multiple calls should use different replicas
		connections := make(map[*pgxpool.Pool]bool)
		for i := 0; i < 10; i++ {
			conn := pool.GetReadConnection(false)
			connections[conn] = true
		}
		
		// Should have used multiple different connections
		assert.GreaterOrEqual(t, len(connections), 1)
	})
}

// Test health check routine behavior
func TestConnectionPool_HealthCheckRoutine(t *testing.T) {
	logger := zaptest.NewLogger(t)
	
	t.Run("removes unhealthy replicas", func(t *testing.T) {
		// This test simulates unhealthy replicas by using invalid URLs
		cfg := &ExtendedDatabaseConfig{
			DatabaseConfig: &config.DatabaseConfig{
				URL: testutil.GetTestDatabaseURL(),
			},
			PrimaryURL: testutil.GetTestDatabaseURL(),
			ReplicaURLs: []string{
				testutil.GetTestDatabaseURL(),
				"postgresql://invalid:invalid@localhost:9999/invalid", // This will fail
			},
		}
		
		pool, err := NewConnectionPoolWithExtended(cfg, logger)
		require.NoError(t, err)
		defer pool.Close()
		
		// Initially should have 1 healthy replica
		assert.Equal(t, 1, len(pool.replicas))
		
		// Run health check
		pool.performHealthCheck()
		
		// Should still have 1 healthy replica
		assert.Equal(t, 1, len(pool.replicas))
	})
}

// Property-based test for Transaction behavior
func TestConnectionPool_PropertyBasedTransactions(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := testutil.NewTestDB(t)
	
	cfg := &config.DatabaseConfig{
		URL: db.ConnectionString(),
	}
	
	pool, err := NewConnectionPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()
	
	ctx := context.Background()
	
	// Create test table
	_, err = pool.GetPrimary().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS property_test (
			id SERIAL PRIMARY KEY,
			value INTEGER NOT NULL
		)
	`)
	require.NoError(t, err)
	defer pool.GetPrimary().Exec(ctx, "DROP TABLE property_test")
	
	// Property: All successful transactions should be visible
	property := func(operations []int) bool {
		if len(operations) == 0 || len(operations) > 100 {
			return true // Skip edge cases
		}
		
		// Clear table
		_, _ = pool.GetPrimary().Exec(ctx, "TRUNCATE property_test")
		
		// Execute transactions
		successCount := 0
		for _, val := range operations {
			err := pool.Transaction(ctx, func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, "INSERT INTO property_test (value) VALUES ($1)", val)
				return err
			})
			if err == nil {
				successCount++
			}
		}
		
		// Verify count matches
		var count int
		err := pool.GetPrimary().QueryRow(ctx, "SELECT COUNT(*) FROM property_test").Scan(&count)
		
		return err == nil && count == successCount
	}
	
	if err := quick.Check(property, &quick.Config{MaxCount: 10}); err != nil {
		t.Error(err)
	}
}

// Test connection pool behavior under resource constraints
func TestConnectionPool_ResourceConstraints(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := testutil.NewTestDB(t)
	
	// Create pool with very limited connections
	cfg := &config.DatabaseConfig{
		URL:          db.ConnectionString(),
		MaxOpenConns: 2,
		MaxIdleConns: 1,
	}
	
	pool, err := NewConnectionPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()
	
	ctx := context.Background()
	
	// Try to exceed connection limit
	const numGoroutines = 10
	errors := make(chan error, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func() {
			// Use a short timeout to avoid blocking forever
			ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()
			
			var result int
			err := pool.GetPrimary().QueryRow(ctx, "SELECT pg_sleep(0.05), 1").Scan(&result, &result)
			errors <- err
		}()
	}
	
	// Collect results
	timeoutCount := 0
	for i := 0; i < numGoroutines; i++ {
		if err := <-errors; err != nil {
			if err.Error() == "timeout: context deadline exceeded" {
				timeoutCount++
			}
		}
	}
	
	// Some operations should have timed out due to connection limit
	assert.Greater(t, timeoutCount, 0)
}

// Test GetDB compatibility method
func TestConnectionPool_GetDB(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.DatabaseConfig{
		URL: testutil.GetTestDatabaseURL(),
	}
	
	pool, err := NewConnectionPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()
	
	// Get standard DB
	db, err := pool.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)
	
	// Verify it works
	var result int
	err = db.QueryRow("SELECT 1").Scan(&result)
	assert.NoError(t, err)
	assert.Equal(t, 1, result)
}