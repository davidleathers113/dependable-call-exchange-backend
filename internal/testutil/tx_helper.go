package testutil

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

// WithTransaction executes a test function within a transaction that's automatically rolled back.
// This ensures complete test isolation by preventing any database changes from persisting.
// 
// Usage:
//   WithTransaction(t, db, func(tx *sql.Tx) {
//       // Your test code here - all changes will be rolled back
//   })
func WithTransaction(t *testing.T, db *sql.DB, fn func(tx *sql.Tx)) {
	t.Helper()

	tx, err := db.Begin()
	require.NoError(t, err, "failed to begin transaction")

	// Always rollback to ensure test isolation
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			t.Errorf("failed to rollback transaction: %v", rbErr)
		}
	}()

	// Execute the test function
	fn(tx)
}

// WithRollback is an alias for WithTransaction for backward compatibility.
// It executes a test function within a transaction that's automatically rolled back.
func WithRollback(t *testing.T, db *sql.DB, fn func(tx *sql.Tx)) {
	WithTransaction(t, db, fn)
}

// WithTransactionContext executes a test function within a transaction with context support.
// The transaction is automatically rolled back to ensure test isolation.
//
// Usage:
//   WithTransactionContext(t, ctx, db, func(ctx context.Context, tx *sql.Tx) {
//       // Your test code here - context is available for cancellation
//   })
func WithTransactionContext(t *testing.T, ctx context.Context, db *sql.DB, fn func(ctx context.Context, tx *sql.Tx)) {
	t.Helper()

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err, "failed to begin transaction")

	// Always rollback to ensure test isolation
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			t.Errorf("failed to rollback transaction: %v", rbErr)
		}
	}()

	// Execute the test function with context
	fn(ctx, tx)
}

// WithParallelTransactions allows running multiple test functions in parallel,
// each within their own isolated transaction. All transactions are rolled back.
//
// Usage:
//   WithParallelTransactions(t, db, 
//       func(tx *sql.Tx) { /* test 1 */ },
//       func(tx *sql.Tx) { /* test 2 */ },
//   )
func WithParallelTransactions(t *testing.T, db *sql.DB, fns ...func(tx *sql.Tx)) {
	t.Helper()

	t.Run("parallel", func(t *testing.T) {
		for i, fn := range fns {
			fn := fn // capture loop variable
			t.Run(string(rune('A'+i)), func(t *testing.T) {
				t.Parallel()
				WithTransaction(t, db, fn)
			})
		}
	})
}

// TxTestFunc is a test function that accepts a transaction
type TxTestFunc func(t *testing.T, tx *sql.Tx)

// RunTransactionalTest is a higher-level helper that combines transaction isolation
// with test setup and assertions.
//
// Usage:
//   RunTransactionalTest(t, db, "test name", func(t *testing.T, tx *sql.Tx) {
//       // Setup
//       // Execute
//       // Assert
//   })
func RunTransactionalTest(t *testing.T, db *sql.DB, name string, fn TxTestFunc) {
	t.Run(name, func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err, "failed to begin transaction")

		// Always rollback
		defer func() {
			if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
				t.Errorf("failed to rollback transaction: %v", rbErr)
			}
		}()

		fn(t, tx)
	})
}

// TransactionTestSuite provides a structured way to run multiple related tests
// within transactions.
type TransactionTestSuite struct {
	t    *testing.T
	db   *sql.DB
	tests map[string]TxTestFunc
}

// NewTransactionTestSuite creates a new test suite for transactional tests
func NewTransactionTestSuite(t *testing.T, db *sql.DB) *TransactionTestSuite {
	return &TransactionTestSuite{
		t:     t,
		db:    db,
		tests: make(map[string]TxTestFunc),
	}
}

// AddTest adds a test to the suite
func (s *TransactionTestSuite) AddTest(name string, fn TxTestFunc) {
	s.tests[name] = fn
}

// Run executes all tests in the suite with transaction isolation
func (s *TransactionTestSuite) Run() {
	for name, fn := range s.tests {
		RunTransactionalTest(s.t, s.db, name, fn)
	}
}

// RunParallel executes all tests in the suite in parallel with transaction isolation
func (s *TransactionTestSuite) RunParallel() {
	for name, fn := range s.tests {
		name, fn := name, fn // capture loop variables
		s.t.Run(name, func(t *testing.T) {
			t.Parallel()
			WithTransaction(t, s.db, func(tx *sql.Tx) {
				fn(t, tx)
			})
		})
	}
}