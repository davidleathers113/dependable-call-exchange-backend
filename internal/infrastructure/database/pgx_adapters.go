package database

import (
	"context"
	
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time interface checks
var (
	_ Rows = (*pgxRowsAdapterImpl)(nil)
	_ Row = (*pgxRowAdapterImpl)(nil)
	_ Result = (*pgxResultAdapterImpl)(nil)
	_ Tx = (*pgxTxAdapterImpl)(nil)
	_ Connection = (*pgxConnectionAdapter)(nil)
)

// Implementation of pgxRowsAdapter
type pgxRowsAdapterImpl struct {
	rows pgx.Rows
}

func (r *pgxRowsAdapterImpl) Next() bool {
	return r.rows.Next()
}

func (r *pgxRowsAdapterImpl) Scan(dest ...interface{}) error {
	return r.rows.Scan(dest...)
}

func (r *pgxRowsAdapterImpl) Close() {
	r.rows.Close()
}

func (r *pgxRowsAdapterImpl) Err() error {
	return r.rows.Err()
}

func (r *pgxRowsAdapterImpl) Values() ([]interface{}, error) {
	return r.rows.Values()
}


// Implementation of pgxRowAdapter
type pgxRowAdapterImpl struct {
	row pgx.Row
}

func (r *pgxRowAdapterImpl) Scan(dest ...interface{}) error {
	return r.row.Scan(dest...)
}


// Implementation of pgxResultAdapter
type pgxResultAdapterImpl struct {
	tag pgconn.CommandTag
}

func (r *pgxResultAdapterImpl) RowsAffected() int64 {
	return r.tag.RowsAffected()
}

func (r *pgxResultAdapterImpl) String() string {
	return r.tag.String()
}


// Implementation of pgxTxAdapter
type pgxTxAdapterImpl struct {
	tx pgx.Tx
}

func (t *pgxTxAdapterImpl) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t *pgxTxAdapterImpl) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

func (t *pgxTxAdapterImpl) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	rows, err := t.tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &pgxRowsAdapterImpl{rows: rows}, nil
}

func (t *pgxTxAdapterImpl) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	row := t.tx.QueryRow(ctx, query, args...)
	return &pgxRowAdapterImpl{row: row}
}

func (t *pgxTxAdapterImpl) Exec(ctx context.Context, query string, args ...interface{}) (Result, error) {
	tag, err := t.tx.Exec(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &pgxResultAdapterImpl{tag: tag}, nil
}

// pgxConnectionAdapter adapts pgx connection to our Connection interface
type pgxConnectionAdapter struct {
	conn interface{} // Can be *pgx.Conn or *pgxpool.Pool
}

func (c *pgxConnectionAdapter) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	var rows pgx.Rows
	var err error
	
	switch conn := c.conn.(type) {
	case *pgx.Conn:
		rows, err = conn.Query(ctx, query, args...)
	case *pgxpool.Pool:
		rows, err = conn.Query(ctx, query, args...)
	default:
		panic("unsupported connection type")
	}
	
	if err != nil {
		return nil, err
	}
	return &pgxRowsAdapterImpl{rows: rows}, nil
}

func (c *pgxConnectionAdapter) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	var row pgx.Row
	
	switch conn := c.conn.(type) {
	case *pgx.Conn:
		row = conn.QueryRow(ctx, query, args...)
	case *pgxpool.Pool:
		row = conn.QueryRow(ctx, query, args...)
	default:
		panic("unsupported connection type")
	}
	
	return &pgxRowAdapterImpl{row: row}
}

func (c *pgxConnectionAdapter) Exec(ctx context.Context, query string, args ...interface{}) (Result, error) {
	var tag pgconn.CommandTag
	var err error
	
	switch conn := c.conn.(type) {
	case *pgx.Conn:
		tag, err = conn.Exec(ctx, query, args...)
	case *pgxpool.Pool:
		tag, err = conn.Exec(ctx, query, args...)
	default:
		panic("unsupported connection type")
	}
	
	if err != nil {
		return nil, err
	}
	return &pgxResultAdapterImpl{tag: tag}, nil
}

func (c *pgxConnectionAdapter) Begin(ctx context.Context) (Tx, error) {
	var tx pgx.Tx
	var err error
	
	switch conn := c.conn.(type) {
	case *pgx.Conn:
		tx, err = conn.Begin(ctx)
	case *pgxpool.Pool:
		tx, err = conn.Begin(ctx)
	default:
		panic("unsupported connection type")
	}
	
	if err != nil {
		return nil, err
	}
	return &pgxTxAdapterImpl{tx: tx}, nil
}