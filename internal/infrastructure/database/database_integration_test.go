// +build integration

package database

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
)

// TestDatabaseIntegration_FullWorkflow tests a complete database workflow
func TestDatabaseIntegration_FullWorkflow(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := testutil.NewTestDB(t)
	defer db.Close()
	
	// Create connection pool with replicas
	cfg := &config.DatabaseConfig{
		PrimaryURL: db.ConnectionString(),
		ReplicaURLs: []string{
			db.ConnectionString(), // Use same DB as replica for testing
		},
		MaxConnections:  50,
		MinConnections:  5,
		MaxConnLifetime: 30 * time.Minute,
	}
	
	pool, err := NewConnectionPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()
	
	// Create repository and monitor
	repo := NewBaseRepository(pool, logger)
	monitor := NewMonitor(pool, logger, nil)
	
	ctx := context.Background()
	
	// Phase 1: Schema Setup
	t.Run("schema_setup", func(t *testing.T) {
		err := db.ExecuteMigrations()
		assert.NoError(t, err)
	})
	
	// Phase 2: Create Test Tables
	t.Run("create_tables", func(t *testing.T) {
		queries := []string{
			`CREATE SCHEMA IF NOT EXISTS integration`,
			`
			CREATE TABLE IF NOT EXISTS integration.accounts (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				email VARCHAR(255) UNIQUE NOT NULL,
				balance DECIMAL(15,4) DEFAULT 0.00,
				status VARCHAR(20) DEFAULT 'active',
				metadata JSONB DEFAULT '{}',
				created_at TIMESTAMPTZ DEFAULT NOW(),
				updated_at TIMESTAMPTZ DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,
			`
			CREATE TABLE IF NOT EXISTS integration.transactions (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				account_id UUID NOT NULL REFERENCES integration.accounts(id),
				amount DECIMAL(15,4) NOT NULL,
				type VARCHAR(20) NOT NULL,
				description TEXT,
				created_at TIMESTAMPTZ DEFAULT NOW()
			)`,
			`
			CREATE TABLE IF NOT EXISTS integration.audit_log (
				id BIGSERIAL PRIMARY KEY,
				table_name VARCHAR(100) NOT NULL,
				operation VARCHAR(20) NOT NULL,
				user_id UUID,
				old_data JSONB,
				new_data JSONB,
				created_at TIMESTAMPTZ DEFAULT NOW()
			)`,
			// Create indexes
			`CREATE INDEX idx_accounts_email ON integration.accounts(email) WHERE deleted_at IS NULL`,
			`CREATE INDEX idx_accounts_status ON integration.accounts(status) WHERE deleted_at IS NULL`,
			`CREATE INDEX idx_transactions_account ON integration.transactions(account_id, created_at DESC)`,
			`CREATE INDEX idx_audit_log_table ON integration.audit_log(table_name, created_at DESC)`,
		}
		
		for _, query := range queries {
			_, err := pool.GetPrimary().Exec(ctx, query)
			assert.NoError(t, err)
		}
	})
	
	// Phase 3: Test Concurrent Operations
	t.Run("concurrent_operations", func(t *testing.T) {
		const numAccounts = 50
		const numWorkers = 10
		
		// Create accounts concurrently
		var wg sync.WaitGroup
		accountIDs := make([]uuid.UUID, numAccounts)
		errors := make(chan error, numAccounts)
		
		for i := 0; i < numAccounts; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				
				accountID := uuid.New()
				email := fmt.Sprintf("user%d@example.com", idx)
				
				err := repo.Transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
					_, err := tx.Exec(ctx, `
						INSERT INTO integration.accounts (id, email, balance)
						VALUES ($1, $2, $3)
					`, accountID, email, 1000.0)
					return err
				})
				
				if err != nil {
					errors <- err
				} else {
					accountIDs[idx] = accountID
				}
			}(i)
		}
		
		wg.Wait()
		close(errors)
		
		// Check for errors
		for err := range errors {
			assert.NoError(t, err)
		}
		
		// Verify all accounts created
		var count int
		err := pool.GetPrimary().QueryRow(ctx, "SELECT COUNT(*) FROM integration.accounts").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, numAccounts, count)
	})
	
	// Phase 4: Test Complex Queries
	t.Run("complex_queries", func(t *testing.T) {
		// Test query builder with joins
		qb := repo.NewQueryBuilder("integration", "transactions").
			Select("t.id", "t.amount", "a.email", "a.balance").
			Join("INNER", "integration.accounts a", "t.account_id = a.id").
			Where("t.amount > ?", 0).
			Where("a.status = ?", "active").
			OrderBy("t.created_at", true).
			Limit(10)
		
		query, args := qb.Build()
		rows, err := repo.ExecuteQuery(ctx, query, args...)
		assert.NoError(t, err)
		rows.Close()
	})
	
	// Phase 5: Test Batch Operations
	t.Run("batch_operations", func(t *testing.T) {
		// Get some account IDs
		var accountIDs []uuid.UUID
		rows, err := pool.GetPrimary().Query(ctx, "SELECT id FROM integration.accounts LIMIT 10")
		require.NoError(t, err)
		
		for rows.Next() {
			var id uuid.UUID
			err := rows.Scan(&id)
			require.NoError(t, err)
			accountIDs = append(accountIDs, id)
		}
		rows.Close()
		
		// Batch insert transactions
		columns := []string{"account_id", "amount", "type", "description"}
		values := make([][]interface{}, 0, len(accountIDs)*10)
		
		for _, accountID := range accountIDs {
			for i := 0; i < 10; i++ {
				values = append(values, []interface{}{
					accountID,
					float64(i * 10),
					"credit",
					fmt.Sprintf("Transaction %d", i),
				})
			}
		}
		
		err = repo.BatchInsert(ctx, "integration", "transactions", columns, values)
		assert.NoError(t, err)
		
		// Verify transactions
		var txCount int
		err = pool.GetPrimary().QueryRow(ctx, "SELECT COUNT(*) FROM integration.transactions").Scan(&txCount)
		assert.NoError(t, err)
		assert.Equal(t, len(values), txCount)
	})
	
	// Phase 6: Test Monitoring
	t.Run("monitoring", func(t *testing.T) {
		// Get connection stats
		connStats, err := monitor.GetConnectionStats(ctx)
		if err == nil {
			assert.NotNil(t, connStats)
			assert.Greater(t, connStats.TotalConnections, 0)
			assert.Greater(t, connStats.MaxConnections, 0)
		}
		
		// Get table stats
		tableStats, err := monitor.GetTableStats(ctx)
		if err == nil {
			assert.NotEmpty(t, tableStats)
			
			// Find our test tables
			var foundAccounts, foundTransactions bool
			for _, stat := range tableStats {
				if stat.SchemaName == "integration" {
					if stat.TableName == "accounts" {
						foundAccounts = true
						assert.Greater(t, stat.LiveTuples, int64(0))
					}
					if stat.TableName == "transactions" {
						foundTransactions = true
						assert.Greater(t, stat.LiveTuples, int64(0))
					}
				}
			}
			assert.True(t, foundAccounts)
			assert.True(t, foundTransactions)
		}
		
		// Run health check
		health, err := monitor.RunHealthCheck(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, health)
		
		overallHealthy, ok := health["overall_healthy"].(bool)
		assert.True(t, ok)
		assert.True(t, overallHealthy)
	})
	
	// Phase 7: Test Streaming
	t.Run("streaming", func(t *testing.T) {
		processedCount := 0
		batchCount := 0
		
		handler := func(batch []interface{}) error {
			batchCount++
			processedCount += len(batch)
			
			// Simulate processing
			time.Sleep(10 * time.Millisecond)
			return nil
		}
		
		query := "SELECT * FROM integration.transactions ORDER BY created_at DESC"
		err := repo.StreamQuery(ctx, query, []interface{}{}, 25, handler)
		
		assert.NoError(t, err)
		assert.Greater(t, processedCount, 0)
		assert.Greater(t, batchCount, 0)
	})
	
	// Phase 8: Test Connection Pool Behavior
	t.Run("connection_pool_stress", func(t *testing.T) {
		const numGoroutines = 100
		const opsPerGoroutine = 50
		
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)
		
		start := time.Now()
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < opsPerGoroutine; j++ {
					// Mix of read and write operations
					var err error
					
					switch j % 4 {
					case 0:
						// Simple read
						var count int
						err = pool.GetReadConnection(false).
							QueryRow(ctx, "SELECT COUNT(*) FROM integration.accounts").
							Scan(&count)
						
					case 1:
						// Read with replica preference
						var balance float64
						err = pool.GetReadConnection(false).
							QueryRow(ctx, "SELECT AVG(balance) FROM integration.accounts").
							Scan(&balance)
						
					case 2:
						// Write in transaction
						err = pool.Transaction(ctx, func(tx pgx.Tx) error {
							_, err := tx.Exec(ctx, `
								UPDATE integration.accounts 
								SET balance = balance + 1,
								    updated_at = NOW()
								WHERE id = (
									SELECT id FROM integration.accounts 
									ORDER BY RANDOM() 
									LIMIT 1
								)
							`)
							return err
						})
						
					case 3:
						// Complex query
						rows, err := pool.GetReadConnection(false).Query(ctx, `
							SELECT a.id, a.email, COUNT(t.id) as tx_count
							FROM integration.accounts a
							LEFT JOIN integration.transactions t ON a.id = t.account_id
							GROUP BY a.id, a.email
							LIMIT 5
						`)
						if err == nil {
							rows.Close()
						}
					}
					
					if err != nil {
						errors <- fmt.Errorf("worker %d op %d: %w", workerID, j, err)
						return
					}
				}
			}(i)
		}
		
		wg.Wait()
		close(errors)
		
		elapsed := time.Since(start)
		totalOps := numGoroutines * opsPerGoroutine
		opsPerSecond := float64(totalOps) / elapsed.Seconds()
		
		t.Logf("Completed %d operations in %v (%.2f ops/sec)", totalOps, elapsed, opsPerSecond)
		
		// Check for errors
		errorCount := 0
		for err := range errors {
			errorCount++
			if errorCount <= 5 { // Log first 5 errors
				t.Logf("Error: %v", err)
			}
		}
		
		assert.Less(t, errorCount, totalOps/100) // Less than 1% error rate
	})
	
	// Phase 9: Test Circuit Breaker
	t.Run("circuit_breaker", func(t *testing.T) {
		// This test would require simulating database failures
		// which is complex in an integration test
		// Verify circuit breaker is initialized
		assert.NotNil(t, pool.circuitBreaker)
		assert.Equal(t, CircuitClosed, pool.circuitBreaker.state)
	})
	
	// Phase 10: Cleanup
	t.Run("cleanup", func(t *testing.T) {
		_, err := pool.GetPrimary().Exec(ctx, "DROP SCHEMA integration CASCADE")
		assert.NoError(t, err)
	})
}