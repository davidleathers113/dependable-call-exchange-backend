package testutil

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
	
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/containers"
)

// EnhancedTestDB provides both traditional and container-based test databases
type EnhancedTestDB struct {
	*TestDB
	container *containers.PostgresContainer
	useContainer bool
}

// NewEnhancedTestDB creates a test database with optional container support
func NewEnhancedTestDB(t *testing.T, opts ...TestOption) *EnhancedTestDB {
	config := &testConfig{
		useContainer: false, // Default to existing infrastructure
	}
	
	for _, opt := range opts {
		opt(config)
	}
	
	if config.useContainer {
		return newContainerTestDB(t)
	}
	
	// Fall back to existing TestDB
	return &EnhancedTestDB{
		TestDB: NewTestDB(t),
		useContainer: false,
	}
}

type testConfig struct {
	useContainer bool
}

type TestOption func(*testConfig)

// WithContainers enables testcontainers for this test
func WithContainers() TestOption {
	return func(c *testConfig) {
		c.useContainer = true
	}
}

func newContainerTestDB(t *testing.T) *EnhancedTestDB {
	ctx := context.Background()
	
	container, err := containers.NewPostgresContainer(ctx)
	require.NoError(t, err)
	
	db, err := sql.Open("postgres", container.ConnectionString)
	require.NoError(t, err)
	
	// Create base TestDB structure
	tdb := &TestDB{
		t:      t,
		db:     db,
		dbName: "dce_test",
	}
	
	// Initialize schema
	tdb.InitSchema()
	
	enhanced := &EnhancedTestDB{
		TestDB:       tdb,
		container:    container,
		useContainer: true,
	}
	
	// Register cleanup
	t.Cleanup(func() {
		db.Close()
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	})
	
	return enhanced
}

// RestoreSnapshot provides fast database restoration for container-based tests
func (e *EnhancedTestDB) RestoreSnapshot() error {
	if !e.useContainer || e.container == nil {
		// Fall back to TruncateTables for non-container tests
		e.TruncateTables()
		return nil
	}
	
	ctx := context.Background()
	return e.container.RestoreSnapshot(ctx)
}

// RunInTransaction executes a function within a transaction that's always rolled back
func (e *EnhancedTestDB) RunInTransaction(fn func(*sql.Tx) error) error {
	tx, err := e.db.Begin()
	if err != nil {
		return err
	}
	
	defer func() {
		// Always rollback to ensure test isolation
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			e.t.Errorf("failed to rollback transaction: %v", rbErr)
		}
	}()
	
	return fn(tx)
}