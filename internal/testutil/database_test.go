package testutil

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTestDB_Testcontainers(t *testing.T) {
	// This will use testcontainers by default
	db := NewTestDB(t)

	// Verify we're using testcontainers
	assert.True(t, db.useContainer)
	assert.NotNil(t, db.container)

	// Test basic query
	var result int
	err := db.DB().QueryRow("SELECT 1").Scan(&result)
	require.NoError(t, err)
	assert.Equal(t, 1, result)

	// Test schema was initialized
	var tableCount int
	err = db.DB().QueryRow(`
		SELECT COUNT(*) 
		FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_type = 'BASE TABLE'
	`).Scan(&tableCount)
	require.NoError(t, err)
	assert.Greater(t, tableCount, 0)
}

func TestTestDB_TruncateTables(t *testing.T) {
	db := NewTestDB(t)

	// Insert test data
	_, err := db.DB().Exec(`
		INSERT INTO accounts (email, company, type, status) 
		VALUES ('test@example.com', 'Test Co', 'buyer', 'active')
	`)
	require.NoError(t, err)

	// Verify data exists
	var count int
	err = db.DB().QueryRow("SELECT COUNT(*) FROM accounts").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Truncate tables
	db.TruncateTables()

	// Verify data is gone
	err = db.DB().QueryRow("SELECT COUNT(*) FROM accounts").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestTestDB_RunInTransaction(t *testing.T) {
	db := NewTestDB(t)

	// Run transaction that inserts data
	err := db.RunInTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO accounts (email, company, type, status) 
			VALUES ('tx@example.com', 'TX Co', 'seller', 'active')
		`)
		return err
	})
	require.NoError(t, err)

	// Verify data was NOT persisted (transaction was rolled back)
	var count int
	err = db.DB().QueryRow("SELECT COUNT(*) FROM accounts WHERE email = 'tx@example.com'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestTestDB_Concurrent(t *testing.T) {
	// Test that multiple test databases can run concurrently
	// This tests container reuse functionality

	const numDatabases = 3
	done := make(chan bool, numDatabases)

	for i := 0; i < numDatabases; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Each goroutine creates its own test database
			db := NewTestDB(t)

			// Perform some operations
			var result int
			err := db.DB().QueryRow("SELECT $1::int", id).Scan(&result)
			assert.NoError(t, err)
			assert.Equal(t, id, result)

			// Small delay to simulate work
			time.Sleep(100 * time.Millisecond)
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < numDatabases; i++ {
		<-done
	}
}

func BenchmarkTestDB_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db := NewTestDB(&testing.T{})
		db.cleanup()
	}
}

func BenchmarkTestDB_TruncateTables(b *testing.B) {
	t := &testing.T{}
	db := NewTestDB(t)

	// Insert some data to truncate
	db.DB().Exec(`
		INSERT INTO accounts (email, company, type, status) 
		SELECT 
			'user' || i || '@example.com',
			'Company ' || i,
			CASE WHEN i % 2 = 0 THEN 'buyer' ELSE 'seller' END,
			'active'
		FROM generate_series(1, 100) i
	`)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		db.TruncateTables()

		// Re-insert data for next iteration
		if i < b.N-1 {
			db.DB().Exec(`
				INSERT INTO accounts (email, company, type, status) 
				SELECT 
					'user' || i || '@example.com',
					'Company ' || i,
					CASE WHEN i % 2 = 0 THEN 'buyer' ELSE 'seller' END,
					'active'
				FROM generate_series(1, 100) i
			`)
		}
	}
}
