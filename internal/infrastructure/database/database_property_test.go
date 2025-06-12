package database

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
)

// Property: Connection pool should handle any valid configuration
func TestConnectionPool_PropertyBasedConfig(t *testing.T) {
	logger := zaptest.NewLogger(t)

	property := func(maxConns, minConns uint8, maxLifetimeMinutes uint16) bool {
		// Skip invalid combinations
		if maxConns == 0 || minConns > maxConns {
			return true
		}

		// Create extended config for property test
		extCfg := &ExtendedDatabaseConfig{
			DatabaseConfig: &config.DatabaseConfig{
				URL:             testutil.GetTestDatabaseURL(),
				MaxOpenConns:    int(maxConns),
				MaxIdleConns:    int(minConns),
				ConnMaxLifetime: time.Duration(maxLifetimeMinutes) * time.Minute,
			},
			PrimaryURL:      testutil.GetTestDatabaseURL(),
			MaxConnections:  int(maxConns),
			MinConnections:  int(minConns),
			MaxConnLifetime: time.Duration(maxLifetimeMinutes) * time.Minute,
		}

		pool, err := NewConnectionPoolWithExtended(extCfg, logger)
		if err != nil {
			return false
		}
		defer pool.Close()

		// Verify pool works
		ctx := context.Background()
		var result int
		err = pool.GetPrimary().QueryRow(ctx, "SELECT 1").Scan(&result)

		return err == nil && result == 1
	}

	if err := quick.Check(property, nil); err != nil {
		t.Error(err)
	}
}

// Property: QueryBuilder should produce valid SQL for any combination of clauses
func TestQueryBuilder_PropertyBased(t *testing.T) {
	logger := zaptest.NewLogger(t)
	pool := &ConnectionPool{}
	repo := NewBaseRepository(pool, logger)

	type QueryInput struct {
		Schema     string
		Table      string
		Columns    []string
		WhereCount int
		JoinCount  int
		OrderCount int
		GroupCount int
		Limit      int
		Offset     int
	}

	// Custom generator for QueryInput
	generateQueryInput := func(rand *rand.Rand) QueryInput {
		schemas := []string{"public", "core", "billing", "analytics"}
		tables := []string{"users", "accounts", "transactions", "logs"}
		columns := []string{"id", "name", "email", "created_at", "status", "amount"}

		input := QueryInput{
			Schema:     schemas[rand.Intn(len(schemas))],
			Table:      tables[rand.Intn(len(tables))],
			WhereCount: rand.Intn(5),
			JoinCount:  rand.Intn(3),
			OrderCount: rand.Intn(4),
			GroupCount: rand.Intn(3),
		}

		// Generate columns
		colCount := rand.Intn(len(columns)) + 1
		input.Columns = make([]string, colCount)
		for i := 0; i < colCount; i++ {
			input.Columns[i] = columns[rand.Intn(len(columns))]
		}

		// Generate limit/offset
		if rand.Float32() < 0.5 {
			input.Limit = rand.Intn(100) + 1
			if rand.Float32() < 0.3 {
				input.Offset = rand.Intn(1000)
			}
		}

		return input
	}

	// Run property test
	for i := 0; i < 1000; i++ {
		input := generateQueryInput(rand.New(rand.NewSource(int64(i))))

		qb := repo.NewQueryBuilder(input.Schema, input.Table)

		// Apply columns
		if len(input.Columns) > 0 {
			qb.Select(input.Columns...)
		}

		// Apply WHERE clauses
		for j := 0; j < input.WhereCount; j++ {
			qb.Where(fmt.Sprintf("column%d = ?", j), fmt.Sprintf("value%d", j))
		}

		// Apply JOINs
		for j := 0; j < input.JoinCount; j++ {
			qb.Join("INNER", fmt.Sprintf("table%d", j), fmt.Sprintf("t.id = table%d.ref_id", j))
		}

		// Apply GROUP BY
		if input.GroupCount > 0 {
			groupCols := make([]string, input.GroupCount)
			for j := 0; j < input.GroupCount; j++ {
				groupCols[j] = fmt.Sprintf("col%d", j)
			}
			qb.GroupBy(groupCols...)
		}

		// Apply ORDER BY
		for j := 0; j < input.OrderCount; j++ {
			qb.OrderBy(fmt.Sprintf("col%d", j), j%2 == 0)
		}

		// Apply LIMIT/OFFSET
		if input.Limit > 0 {
			qb.Limit(input.Limit)
		}
		if input.Offset > 0 {
			qb.Offset(input.Offset)
		}

		// Build query
		query, args := qb.Build()

		// Verify query is valid SQL structure
		assert.True(t, strings.HasPrefix(query, "SELECT"))
		assert.Contains(t, query, "FROM")
		assert.Contains(t, query, input.Schema+"."+input.Table)

		// Verify argument count matches placeholders
		placeholderCount := strings.Count(query, "$")
		assert.Equal(t, placeholderCount, len(args))

		// Verify query structure
		if input.WhereCount > 0 {
			assert.Contains(t, query, "WHERE")
		}
		if input.JoinCount > 0 {
			assert.Contains(t, query, "JOIN")
		}
		if input.GroupCount > 0 {
			assert.Contains(t, query, "GROUP BY")
		}
		if input.OrderCount > 0 {
			assert.Contains(t, query, "ORDER BY")
		}
		if input.Limit > 0 {
			assert.Contains(t, query, "LIMIT")
		}
		if input.Offset > 0 {
			assert.Contains(t, query, "OFFSET")
		}
	}
}

// Property: BatchInsert should handle any valid batch size and data
func TestBatchInsert_PropertyBased(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := testutil.NewTestDB(t)
	// No need to defer Close - TestDB uses t.Cleanup

	cfg := &config.DatabaseConfig{
		URL: db.ConnectionString(),
	}

	pool, err := NewConnectionPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()

	repo := NewBaseRepository(pool, logger)
	ctx := context.Background()

	// Create test table
	_, err = pool.GetPrimary().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.batch_property_test (
			id UUID PRIMARY KEY,
			name TEXT NOT NULL,
			value INT NOT NULL,
			active BOOLEAN NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	require.NoError(t, err)
	defer pool.GetPrimary().Exec(ctx, "DROP TABLE public.batch_property_test")

	property := func(batchSize uint8, nameLength uint8, maxValue int32) bool {
		// Limit ranges for practical testing
		if batchSize == 0 || batchSize > 100 || nameLength == 0 || nameLength > 50 {
			return true
		}

		// Clear table
		pool.GetPrimary().Exec(ctx, "TRUNCATE public.batch_property_test")

		// Generate batch data
		columns := []string{"id", "name", "value", "active"}
		values := make([][]interface{}, batchSize)

		for i := 0; i < int(batchSize); i++ {
			// Generate random name
			name := strings.Repeat("x", int(nameLength))
			value := rand.Int31n(maxValue + 1)
			active := rand.Float32() < 0.5

			values[i] = []interface{}{
				uuid.New(),
				name,
				value,
				active,
			}
		}

		// Execute batch insert
		err := repo.BatchInsert(ctx, "public", "batch_property_test", columns, values)
		if err != nil {
			return false
		}

		// Verify count
		var count int
		err = pool.GetPrimary().QueryRow(ctx, "SELECT COUNT(*) FROM public.batch_property_test").Scan(&count)

		return err == nil && count == int(batchSize)
	}

	config := &quick.Config{
		MaxCount: 100,
	}
	if err := quick.Check(property, config); err != nil {
		t.Error(err)
	}
}

// Property: Transaction isolation should prevent dirty reads
func TestTransaction_PropertyIsolation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := testutil.NewTestDB(t)
	// No need to defer Close - TestDB uses t.Cleanup

	cfg := &config.DatabaseConfig{
		URL: db.ConnectionString(),
	}

	pool, err := NewConnectionPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()

	repo := NewBaseRepository(pool, logger)
	ctx := context.Background()

	// Create test table
	_, err = pool.GetPrimary().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.isolation_test (
			id SERIAL PRIMARY KEY,
			value INT NOT NULL
		)
	`)
	require.NoError(t, err)
	defer pool.GetPrimary().Exec(ctx, "DROP TABLE public.isolation_test")

	property := func(initialValue, increment int16) bool {
		// Clear and insert initial value
		pool.GetPrimary().Exec(ctx, "TRUNCATE public.isolation_test")
		pool.GetPrimary().Exec(ctx, "INSERT INTO public.isolation_test (value) VALUES ($1)", initialValue)

		// Start transaction that will be rolled back
		txErr := make(chan error, 1)
		go func() {
			err := repo.Transaction(ctx, func(ctx context.Context, tx Tx) error {
				// Update value in transaction
				_, err := tx.Exec(ctx, "UPDATE public.isolation_test SET value = value + $1", increment)
				if err != nil {
					return err
				}

				// Sleep to ensure other goroutine reads during transaction
				time.Sleep(50 * time.Millisecond)

				// Rollback
				return fmt.Errorf("intentional rollback")
			})
			txErr <- err
		}()

		// Wait a bit then read value from another connection
		time.Sleep(25 * time.Millisecond)

		var readValue int
		err := pool.GetPrimary().QueryRow(ctx, "SELECT value FROM public.isolation_test").Scan(&readValue)
		if err != nil {
			return false
		}

		// Wait for transaction to complete
		<-txErr

		// Value should not have changed (transaction was rolled back)
		return readValue == int(initialValue)
	}

	config := &quick.Config{
		MaxCount: 50, // Reduced due to sleep times
	}
	if err := quick.Check(property, config); err != nil {
		t.Error(err)
	}
}

// Property: Circuit breaker should prevent cascading failures
func TestCircuitBreaker_PropertyBased(t *testing.T) {
	property := func(threshold uint8, failureCount uint16, successCount uint8) bool {
		// Reasonable bounds
		if threshold == 0 || threshold > 20 {
			return true
		}

		cb := &CircuitBreaker{
			timeout:   50 * time.Millisecond,
			threshold: int(threshold),
			state:     CircuitClosed,
		}

		// Record failures
		actualFailures := int(failureCount) % (int(threshold) * 2) // Bound the failures
		for i := 0; i < actualFailures; i++ {
			cb.RecordFailure()
		}

		// Check state
		if actualFailures >= int(threshold) {
			// Should be open
			if cb.state != CircuitOpen {
				return false
			}
			if cb.Allow() {
				return false
			}

			// After timeout, should transition to half-open
			time.Sleep(cb.timeout + 10*time.Millisecond)
			if !cb.Allow() {
				return false
			}
			if cb.state != CircuitHalfOpen {
				return false
			}
		}

		// Record successes
		for i := 0; i < int(successCount); i++ {
			cb.RecordSuccess()
		}

		// After any success, should be closed
		if successCount > 0 {
			return cb.state == CircuitClosed && cb.Allow()
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}
	if err := quick.Check(property, config); err != nil {
		t.Error(err)
	}
}

// Property: Stream processing should handle any batch size correctly
func TestStreamQuery_PropertyBased(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := testutil.NewTestDB(t)
	// No need to defer Close - TestDB uses t.Cleanup

	cfg := &config.DatabaseConfig{
		URL: db.ConnectionString(),
	}

	pool, err := NewConnectionPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()

	repo := NewBaseRepository(pool, logger)
	ctx := context.Background()

	// Create test table
	_, err = pool.GetPrimary().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.stream_property_test (
			id SERIAL PRIMARY KEY,
			value INT NOT NULL
		)
	`)
	require.NoError(t, err)
	defer pool.GetPrimary().Exec(ctx, "DROP TABLE public.stream_property_test")

	property := func(totalRows uint16, batchSize uint8) bool {
		// Reasonable bounds
		if totalRows == 0 || totalRows > 1000 || batchSize == 0 {
			return true
		}

		// Clear and insert data
		pool.GetPrimary().Exec(ctx, "TRUNCATE public.stream_property_test")
		for i := 0; i < int(totalRows); i++ {
			pool.GetPrimary().Exec(ctx, "INSERT INTO public.stream_property_test (value) VALUES ($1)", i)
		}

		// Stream and collect results
		var totalProcessed int
		var batchCount int

		handler := func(batch []interface{}) error {
			batchCount++
			totalProcessed += len(batch)

			// Verify batch size (except possibly last batch)
			if totalProcessed < int(totalRows) {
				if len(batch) != int(batchSize) {
					return fmt.Errorf("unexpected batch size: got %d, want %d", len(batch), batchSize)
				}
			}

			return nil
		}

		query := "SELECT id, value FROM public.stream_property_test ORDER BY id"
		err := repo.StreamQuery(ctx, query, []interface{}{}, int(batchSize), handler)

		if err != nil {
			return false
		}

		// Verify all rows were processed
		if totalProcessed != int(totalRows) {
			return false
		}

		// Verify batch count
		expectedBatches := (int(totalRows) + int(batchSize) - 1) / int(batchSize)
		return batchCount == expectedBatches
	}

	config := &quick.Config{
		MaxCount: 50,
	}
	if err := quick.Check(property, config); err != nil {
		t.Error(err)
	}
}

// Generate implements quick.Generator for ConnectionPool testing
type ConnectionConfig struct {
	MaxConns        int
	MinConns        int
	MaxLifetime     time.Duration
	MaxIdleTime     time.Duration
	HealthCheckTime time.Duration
}

func (ConnectionConfig) Generate(rand *rand.Rand, size int) reflect.Value {
	cc := ConnectionConfig{
		MaxConns:        rand.Intn(100) + 1,
		MinConns:        rand.Intn(20),
		MaxLifetime:     time.Duration(rand.Intn(3600)) * time.Second,
		MaxIdleTime:     time.Duration(rand.Intn(600)) * time.Second,
		HealthCheckTime: time.Duration(rand.Intn(300)) * time.Second,
	}

	// Ensure min <= max
	if cc.MinConns > cc.MaxConns {
		cc.MinConns = cc.MaxConns / 2
	}

	return reflect.ValueOf(cc)
}
