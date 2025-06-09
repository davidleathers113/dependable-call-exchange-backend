package database

import (
	"context"
)

// Rows abstracts database rows iteration
type Rows interface {
	// Next prepares the next row for reading
	Next() bool
	
	// Scan reads the current row into dest values
	Scan(dest ...interface{}) error
	
	// Close closes the rows
	Close()
	
	// Err returns any error that occurred during iteration
	Err() error
	
	// Values returns the current row values
	Values() ([]interface{}, error)
}

// Row abstracts a single database row
type Row interface {
	// Scan reads the row into dest values
	Scan(dest ...interface{}) error
}

// Result abstracts command execution result
type Result interface {
	// RowsAffected returns number of rows affected
	RowsAffected() int64
	
	// String returns string representation
	String() string
}

// Tx abstracts database transaction
type Tx interface {
	// Commit commits the transaction
	Commit(ctx context.Context) error
	
	// Rollback rolls back the transaction
	Rollback(ctx context.Context) error
	
	// Query executes a query within the transaction
	Query(ctx context.Context, query string, args ...interface{}) (Rows, error)
	
	// QueryRow executes a query returning single row
	QueryRow(ctx context.Context, query string, args ...interface{}) Row
	
	// Exec executes a command within the transaction
	Exec(ctx context.Context, query string, args ...interface{}) (Result, error)
}

// Connection abstracts database connection
type Connection interface {
	// Query executes a query
	Query(ctx context.Context, query string, args ...interface{}) (Rows, error)
	
	// QueryRow executes a query returning single row
	QueryRow(ctx context.Context, query string, args ...interface{}) Row
	
	// Exec executes a command
	Exec(ctx context.Context, query string, args ...interface{}) (Result, error)
	
	// Begin starts a transaction
	Begin(ctx context.Context) (Tx, error)
}

// ErrNoRows is returned when no rows are found
type ErrNoRows struct{}

func (e ErrNoRows) Error() string {
	return "no rows in result set"
}

// NoRows is the standard error for no rows found
var NoRowsError = ErrNoRows{}

// TransactionFunc is a function that executes within a transaction
type TransactionFunc func(ctx context.Context, tx Tx) error