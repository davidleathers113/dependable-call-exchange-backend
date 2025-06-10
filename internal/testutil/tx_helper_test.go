package testutil

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithTransaction(t *testing.T) {
	testDB := NewTestDB(t)
	db := testDB.DB()

	// Insert data in transaction and verify it's rolled back
	t.Run("rollback on success", func(t *testing.T) {
		// Check initial count
		initialCount := testDB.GetRowCount(t, "accounts")

		WithTransaction(t, db, func(tx *sql.Tx) {
			// Insert test data
			_, err := tx.Exec(`
				INSERT INTO accounts (email, name, type, status) 
				VALUES ('test@example.com', 'Test User', 'buyer', 'active')
			`)
			require.NoError(t, err)

			// Verify data exists within transaction
			var count int
			err = tx.QueryRow("SELECT COUNT(*) FROM accounts WHERE email = 'test@example.com'").Scan(&count)
			require.NoError(t, err)
			assert.Equal(t, 1, count, "should see inserted data within transaction")
		})

		// Verify data was rolled back
		finalCount := testDB.GetRowCount(t, "accounts")
		assert.Equal(t, initialCount, finalCount, "transaction should be rolled back")
	})

	t.Run("rollback on panic", func(t *testing.T) {
		initialCount := testDB.GetRowCount(t, "accounts")

		// Use assert.Panics to verify panic is propagated
		assert.Panics(t, func() {
			WithTransaction(t, db, func(tx *sql.Tx) {
				// Insert test data
				_, err := tx.Exec(`
					INSERT INTO accounts (email, name, type, status) 
					VALUES ('panic@example.com', 'Panic User', 'buyer', 'active')
				`)
				require.NoError(t, err)

				// Simulate panic
				panic("test panic")
			})
		})

		// Verify data was rolled back despite panic
		finalCount := testDB.GetRowCount(t, "accounts")
		assert.Equal(t, initialCount, finalCount, "transaction should be rolled back after panic")
	})
}

func TestWithTransactionContext(t *testing.T) {
	testDB := NewTestDB(t)
	db := testDB.DB()

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		
		WithTransactionContext(t, ctx, db, func(ctx context.Context, tx *sql.Tx) {
			// Insert test data
			_, err := tx.ExecContext(ctx, `
				INSERT INTO accounts (email, name, type, status) 
				VALUES ('context@example.com', 'Context User', 'buyer', 'active')
			`)
			require.NoError(t, err)

			// Cancel context
			cancel()

			// Subsequent operations should respect context
			_, err = tx.ExecContext(ctx, `
				INSERT INTO accounts (email, name, type, status) 
				VALUES ('context2@example.com', 'Context User 2', 'buyer', 'active')
			`)
			assert.Error(t, err, "should fail due to cancelled context")
		})

		// Verify all data was rolled back
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM accounts WHERE email LIKE 'context%'").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "all changes should be rolled back")
	})
}

func TestWithParallelTransactions(t *testing.T) {
	testDB := NewTestDB(t)
	db := testDB.DB()

	var wg sync.WaitGroup
	results := make([]bool, 3)

	WithParallelTransactions(t, db,
		func(tx *sql.Tx) {
			wg.Add(1)
			defer wg.Done()
			
			// Transaction 1: Insert buyer
			_, err := tx.Exec(`
				INSERT INTO accounts (email, name, type, status) 
				VALUES ('parallel1@example.com', 'Parallel 1', 'buyer', 'active')
			`)
			require.NoError(t, err)
			results[0] = true
		},
		func(tx *sql.Tx) {
			wg.Add(1)
			defer wg.Done()
			
			// Transaction 2: Insert seller
			_, err := tx.Exec(`
				INSERT INTO accounts (email, name, type, status) 
				VALUES ('parallel2@example.com', 'Parallel 2', 'seller', 'active')
			`)
			require.NoError(t, err)
			results[1] = true
		},
		func(tx *sql.Tx) {
			wg.Add(1)
			defer wg.Done()
			
			// Transaction 3: Insert admin
			_, err := tx.Exec(`
				INSERT INTO accounts (email, name, type, status) 
				VALUES ('parallel3@example.com', 'Parallel 3', 'admin', 'active')
			`)
			require.NoError(t, err)
			results[2] = true
		},
	)

	// Wait for all parallel transactions
	wg.Wait()

	// Verify all transactions executed
	for i, result := range results {
		assert.True(t, result, "transaction %d should have executed", i+1)
	}

	// Verify all data was rolled back
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM accounts WHERE email LIKE 'parallel%'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "all parallel transactions should be rolled back")
}

func TestTransactionTestSuite(t *testing.T) {
	testDB := NewTestDB(t)
	db := testDB.DB()

	suite := NewTransactionTestSuite(t, db)

	// Add multiple related tests
	suite.AddTest("create account", func(t *testing.T, tx *sql.Tx) {
		_, err := tx.Exec(`
			INSERT INTO accounts (email, name, type, status) 
			VALUES ('suite1@example.com', 'Suite 1', 'buyer', 'active')
		`)
		require.NoError(t, err)

		// Verify within transaction
		var count int
		err = tx.QueryRow("SELECT COUNT(*) FROM accounts WHERE email = 'suite1@example.com'").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	suite.AddTest("update account", func(t *testing.T, tx *sql.Tx) {
		// First insert
		_, err := tx.Exec(`
			INSERT INTO accounts (email, name, type, status) 
			VALUES ('suite2@example.com', 'Suite 2', 'seller', 'active')
		`)
		require.NoError(t, err)

		// Then update
		_, err = tx.Exec(`
			UPDATE accounts SET status = 'suspended' 
			WHERE email = 'suite2@example.com'
		`)
		require.NoError(t, err)

		// Verify update
		var status string
		err = tx.QueryRow("SELECT status FROM accounts WHERE email = 'suite2@example.com'").Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "suspended", status)
	})

	// Run the suite
	suite.Run()

	// Verify all changes were rolled back
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM accounts WHERE email LIKE 'suite%'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "all suite tests should be rolled back")
}

func TestTransactionIsolation(t *testing.T) {
	testDB := NewTestDB(t)
	db := testDB.DB()

	// Test that concurrent transactions don't see each other's changes
	t.Run("concurrent isolation", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(2)

		// Channel to coordinate timing
		ch := make(chan bool)

		// Transaction 1
		go func() {
			defer wg.Done()
			WithTransaction(t, db, func(tx1 *sql.Tx) {
				// Insert in tx1
				_, err := tx1.Exec(`
					INSERT INTO accounts (email, name, type, status) 
					VALUES ('isolation1@example.com', 'Isolation 1', 'buyer', 'active')
				`)
				require.NoError(t, err)

				// Signal tx2 to proceed
				ch <- true

				// Wait for tx2 to check
				<-ch

				// Verify own data is visible
				var count int
				err = tx1.QueryRow("SELECT COUNT(*) FROM accounts WHERE email = 'isolation1@example.com'").Scan(&count)
				require.NoError(t, err)
				assert.Equal(t, 1, count, "should see own data")
			})
		}()

		// Transaction 2
		go func() {
			defer wg.Done()
			WithTransaction(t, db, func(tx2 *sql.Tx) {
				// Wait for tx1 to insert
				<-ch

				// Try to see tx1's data (shouldn't be visible)
				var count int
				err := tx2.QueryRow("SELECT COUNT(*) FROM accounts WHERE email = 'isolation1@example.com'").Scan(&count)
				require.NoError(t, err)
				assert.Equal(t, 0, count, "should not see other transaction's uncommitted data")

				// Signal tx1 to continue
				ch <- true
			})
		}()

		wg.Wait()
	})
}

// Benchmark transaction helper performance
func BenchmarkWithTransaction(b *testing.B) {
	// Skip if not running benchmarks
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	testDB := NewTestDB(&testing.T{})
	db := testDB.DB()
	defer testDB.cleanup()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			WithTransaction(&testing.T{}, db, func(tx *sql.Tx) {
				_, _ = tx.Exec(`
					INSERT INTO accounts (email, name, type, status) 
					VALUES ('bench@example.com', 'Bench User', 'buyer', 'active')
				`)
			})
		}
	})
}

// Test helper for deadline scenarios
func TestTransactionDeadline(t *testing.T) {
	testDB := NewTestDB(t)
	db := testDB.DB()

	t.Run("transaction with deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		WithTransactionContext(t, ctx, db, func(ctx context.Context, tx *sql.Tx) {
			// Start a long-running operation
			go func() {
				time.Sleep(200 * time.Millisecond)
				_, _ = tx.ExecContext(ctx, `
					INSERT INTO accounts (email, name, type, status) 
					VALUES ('deadline@example.com', 'Deadline User', 'buyer', 'active')
				`)
			}()

			// Wait for context to expire
			<-ctx.Done()
			assert.Equal(t, context.DeadlineExceeded, ctx.Err())
		})

		// Verify no data was committed
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM accounts WHERE email = 'deadline@example.com'").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "deadline transaction should be rolled back")
	})
}