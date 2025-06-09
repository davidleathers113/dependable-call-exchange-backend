package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
)

// Mock cache for testing
type mockCache struct {
	mock.Mock
}

func (m *mockCache) Get(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *mockCache) Clear(ctx context.Context, pattern string) error {
	args := m.Called(ctx, pattern)
	return args.Error(0)
}

// Mock tracer for testing
type mockTracer struct {
	mock.Mock
}

func (m *mockTracer) StartSpan(ctx context.Context, name string) (context.Context, func()) {
	args := m.Called(ctx, name)
	return args.Get(0).(context.Context), args.Get(1).(func())
}

func TestQueryBuilder(t *testing.T) {
	logger := zaptest.NewLogger(t)
	pool := &ConnectionPool{} // Minimal pool for QueryBuilder
	repo := NewBaseRepository(pool, logger)
	
	t.Run("basic select", func(t *testing.T) {
		qb := repo.NewQueryBuilder("public", "users")
		query, args := qb.Build()
		
		assert.Equal(t, "SELECT * FROM public.users", query)
		assert.Empty(t, args)
	})
	
	t.Run("select with columns", func(t *testing.T) {
		qb := repo.NewQueryBuilder("public", "users").
			Select("id", "email", "name")
		
		query, args := qb.Build()
		
		assert.Equal(t, "SELECT id, email, name FROM public.users", query)
		assert.Empty(t, args)
	})
	
	t.Run("with where clause", func(t *testing.T) {
		qb := repo.NewQueryBuilder("public", "users").
			Where("status = ?", "active").
			Where("created_at > ?", time.Now())
		
		query, args := qb.Build()
		
		assert.Contains(t, query, "WHERE status = $1 AND created_at > $2")
		assert.Len(t, args, 2)
		assert.Equal(t, "active", args[0])
	})
	
	t.Run("with join", func(t *testing.T) {
		qb := repo.NewQueryBuilder("public", "users").
			Join("INNER", "public.accounts", "users.account_id = accounts.id").
			Where("accounts.balance > ?", 100)
		
		query, args := qb.Build()
		
		assert.Contains(t, query, "INNER JOIN public.accounts ON users.account_id = accounts.id")
		assert.Contains(t, query, "WHERE accounts.balance > $1")
		assert.Len(t, args, 1)
		assert.Equal(t, 100, args[0])
	})
	
	t.Run("with order and limit", func(t *testing.T) {
		qb := repo.NewQueryBuilder("public", "users").
			OrderBy("created_at", true).
			OrderBy("name", false).
			Limit(10).
			Offset(20)
		
		query, _ := qb.Build()
		
		assert.Contains(t, query, "ORDER BY created_at DESC, name")
		assert.Contains(t, query, "LIMIT 10")
		assert.Contains(t, query, "OFFSET 20")
	})
	
	t.Run("with group by", func(t *testing.T) {
		qb := repo.NewQueryBuilder("public", "orders").
			Select("user_id", "COUNT(*) as order_count").
			GroupBy("user_id").
			OrderBy("order_count", true)
		
		query, _ := qb.Build()
		
		assert.Contains(t, query, "GROUP BY user_id")
		assert.Contains(t, query, "ORDER BY order_count DESC")
	})
}

func TestBaseRepository_GetByID(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := testutil.NewTestDB(t)
	// TestDB uses t.Cleanup, no need to close manually
	
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
		CREATE TABLE IF NOT EXISTS public.test_entities (
			id UUID PRIMARY KEY,
			name TEXT NOT NULL,
			value INT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		)
	`)
	require.NoError(t, err)
	defer pool.GetPrimary().Exec(ctx, "DROP TABLE public.test_entities")
	
	// Insert test data
	testID := uuid.New()
	testName := "Test Entity"
	testValue := 42
	
	_, err = pool.GetPrimary().Exec(ctx, `
		INSERT INTO public.test_entities (id, name, value)
		VALUES ($1, $2, $3)
	`, testID, testName, testValue)
	require.NoError(t, err)
	
	t.Run("successful retrieval without cache", func(t *testing.T) {
		// Use a scanner function since we can't scan directly into a struct
		var id uuid.UUID
		var name string
		var value int
		var createdAt time.Time
		var deletedAt sql.NullTime
		
		scanner := func(row pgx.Row) error {
			return row.Scan(&id, &name, &value, &createdAt, &deletedAt)
		}
		
		query := "SELECT * FROM public.test_entities WHERE id = $1 AND deleted_at IS NULL"
		row := repo.ExecuteQueryRow(ctx, query, testID)
		err := scanner(row)
		
		assert.NoError(t, err)
		assert.Equal(t, testID, id)
		assert.Equal(t, testName, name)
		assert.Equal(t, testValue, value)
	})
	
	t.Run("with cache", func(t *testing.T) {
		cache := new(mockCache)
		repo.WithCache(cache)
		
		// First call - cache miss
		cacheKey := fmt.Sprintf("public:test_entities:%s", testID)
		cache.On("Get", ctx, cacheKey).Return(nil, fmt.Errorf("not found")).Once()
		
		// Cache set expectation
		cache.On("Set", ctx, cacheKey, mock.Anything, 5*time.Minute).Return(nil).Once()
		
		// Execute query
		var id uuid.UUID
		query := "SELECT id FROM public.test_entities WHERE id = $1 AND deleted_at IS NULL"
		row := repo.ExecuteQueryRow(ctx, query, testID)
		err := row.Scan(&id)
		
		assert.NoError(t, err)
		assert.Equal(t, testID, id)
		
		// Second call - cache hit
		cachedData, _ := json.Marshal(testID)
		cache.On("Get", ctx, cacheKey).Return(cachedData, nil).Once()
		
		// Note: In real implementation, GetByID would handle caching,
		// but we're testing the pattern here
		cache.AssertExpectations(t)
	})
	
	t.Run("not found", func(t *testing.T) {
		nonExistentID := uuid.New()
		
		query := "SELECT * FROM public.test_entities WHERE id = $1 AND deleted_at IS NULL"
		row := repo.ExecuteQueryRow(ctx, query, nonExistentID)
		
		var id uuid.UUID
		err := row.Scan(&id)
		assert.Error(t, err)
		assert.Equal(t, pgx.ErrNoRows, err)
	})
}

func TestBaseRepository_BatchInsert(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := testutil.NewTestDB(t)
	// TestDB uses t.Cleanup, no need to close manually
	
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
		CREATE TABLE IF NOT EXISTS public.batch_test (
			id UUID PRIMARY KEY,
			name TEXT NOT NULL,
			value INT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	require.NoError(t, err)
	defer pool.GetPrimary().Exec(ctx, "DROP TABLE public.batch_test")
	
	t.Run("successful batch insert", func(t *testing.T) {
		// Prepare test data
		columns := []string{"id", "name", "value"}
		values := make([][]interface{}, 100)
		
		for i := 0; i < 100; i++ {
			values[i] = []interface{}{
				uuid.New(),
				fmt.Sprintf("Item %d", i),
				i * 10,
			}
		}
		
		// Execute batch insert
		err := repo.BatchInsert(ctx, "public", "batch_test", columns, values)
		assert.NoError(t, err)
		
		// Verify data was inserted
		var count int
		err = pool.GetPrimary().QueryRow(ctx, "SELECT COUNT(*) FROM public.batch_test").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 100, count)
	})
	
	t.Run("empty batch", func(t *testing.T) {
		columns := []string{"id", "name", "value"}
		values := [][]interface{}{}
		
		err := repo.BatchInsert(ctx, "public", "batch_test", columns, values)
		assert.NoError(t, err) // Should succeed with no-op
	})
	
	t.Run("constraint violation", func(t *testing.T) {
		duplicateID := uuid.New()
		
		// Insert first record
		columns := []string{"id", "name", "value"}
		values := [][]interface{}{
			{duplicateID, "First", 1},
		}
		
		err := repo.BatchInsert(ctx, "public", "batch_test", columns, values)
		assert.NoError(t, err)
		
		// Try to insert duplicate
		values = [][]interface{}{
			{duplicateID, "Duplicate", 2},
		}
		
		err = repo.BatchInsert(ctx, "public", "batch_test", columns, values)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate key")
	})
}

func TestBaseRepository_Transaction(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := testutil.NewTestDB(t)
	// TestDB uses t.Cleanup, no need to close manually
	
	cfg := &config.DatabaseConfig{
		URL: db.ConnectionString(),
	}
	
	pool, err := NewConnectionPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()
	
	repo := NewBaseRepository(pool, logger)
	ctx := context.Background()
	
	// Add tracer
	tracer := new(mockTracer)
	repo.WithTracer(tracer)
	
	// Setup tracer expectations
	tracer.On("StartSpan", ctx, "database.transaction").Return(ctx, func() {})
	
	// Create test table
	_, err = pool.GetPrimary().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.tx_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)
	defer pool.GetPrimary().Exec(ctx, "DROP TABLE public.tx_test")
	
	t.Run("successful transaction", func(t *testing.T) {
		err := repo.Transaction(ctx, func(ctx context.Context, tx Tx) error {
			// Verify we have transaction in context
			txFromCtx := ctx.Value("tx")
			assert.NotNil(t, txFromCtx)
			assert.Equal(t, tx, txFromCtx)
			
			// Execute operations
			_, err := tx.Exec(ctx, "INSERT INTO public.tx_test (value) VALUES ($1)", "test1")
			if err != nil {
				return err
			}
			
			_, err = tx.Exec(ctx, "INSERT INTO public.tx_test (value) VALUES ($1)", "test2")
			return err
		})
		
		assert.NoError(t, err)
		
		// Verify data was committed
		var count int
		err = pool.GetPrimary().QueryRow(ctx, "SELECT COUNT(*) FROM public.tx_test").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 2, count)
	})
	
	t.Run("transaction rollback on error", func(t *testing.T) {
		// Clear previous data
		_, _ = pool.GetPrimary().Exec(ctx, "TRUNCATE public.tx_test")
		
		err := repo.Transaction(ctx, func(ctx context.Context, tx Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO public.tx_test (value) VALUES ($1)", "test3")
			if err != nil {
				return err
			}
			
			// Force an error
			return fmt.Errorf("intentional rollback")
		})
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "intentional rollback")
		
		// Verify data was NOT committed
		var count int
		err = pool.GetPrimary().QueryRow(ctx, "SELECT COUNT(*) FROM public.tx_test").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
	})
	
	tracer.AssertExpectations(t)
}

func TestBaseRepository_StreamQuery(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := testutil.NewTestDB(t)
	// TestDB uses t.Cleanup, no need to close manually
	
	cfg := &config.DatabaseConfig{
		URL: db.ConnectionString(),
	}
	
	pool, err := NewConnectionPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()
	
	repo := NewBaseRepository(pool, logger)
	ctx := context.Background()
	
	// Create test table with data
	_, err = pool.GetPrimary().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.stream_test (
			id SERIAL PRIMARY KEY,
			value INT NOT NULL
		)
	`)
	require.NoError(t, err)
	defer pool.GetPrimary().Exec(ctx, "DROP TABLE public.stream_test")
	
	// Insert test data
	for i := 1; i <= 100; i++ {
		_, err = pool.GetPrimary().Exec(ctx, "INSERT INTO public.stream_test (value) VALUES ($1)", i)
		require.NoError(t, err)
	}
	
	t.Run("stream with batches", func(t *testing.T) {
		var batches [][]interface{}
		batchSize := 10
		
		handler := func(batch []interface{}) error {
			batches = append(batches, batch)
			return nil
		}
		
		query := "SELECT id, value FROM public.stream_test ORDER BY id"
		err := repo.StreamQuery(ctx, query, []interface{}{}, batchSize, handler)
		
		assert.NoError(t, err)
		assert.Len(t, batches, 10) // 100 rows / 10 per batch
		
		// Verify first batch
		assert.Len(t, batches[0], 10)
		
		// Verify last batch
		assert.Len(t, batches[9], 10)
	})
	
	t.Run("stream with partial last batch", func(t *testing.T) {
		var batches [][]interface{}
		batchSize := 15
		
		handler := func(batch []interface{}) error {
			batches = append(batches, batch)
			return nil
		}
		
		query := "SELECT id, value FROM public.stream_test ORDER BY id"
		err := repo.StreamQuery(ctx, query, []interface{}{}, batchSize, handler)
		
		assert.NoError(t, err)
		assert.Len(t, batches, 7) // 100 rows / 15 per batch = 6 full + 1 partial
		assert.Len(t, batches[6], 10) // Last batch has 10 items
	})
	
	t.Run("handler error stops streaming", func(t *testing.T) {
		var processedBatches int
		
		handler := func(batch []interface{}) error {
			processedBatches++
			if processedBatches >= 3 {
				return fmt.Errorf("handler error")
			}
			return nil
		}
		
		query := "SELECT id, value FROM public.stream_test ORDER BY id"
		err := repo.StreamQuery(ctx, query, []interface{}{}, 10, handler)
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handler error")
		assert.Equal(t, 3, processedBatches)
	})
}

func TestBaseRepository_HealthCheck(t *testing.T) {
	logger := zaptest.NewLogger(t)
	db := testutil.NewTestDB(t)
	// TestDB uses t.Cleanup, no need to close manually
	
	cfg := &config.DatabaseConfig{
		URL: db.ConnectionString(),
	}
	
	pool, err := NewConnectionPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()
	
	repo := NewBaseRepository(pool, logger)
	ctx := context.Background()
	
	t.Run("successful health check", func(t *testing.T) {
		err := repo.HealthCheck(ctx)
		assert.NoError(t, err)
	})
	
	t.Run("health check with timeout", func(t *testing.T) {
		// Create a context that's already cancelled
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		
		err := repo.HealthCheck(ctx)
		assert.Error(t, err)
	})
}

func TestBaseRepository_CacheInvalidation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	pool := &ConnectionPool{} // Minimal pool
	repo := NewBaseRepository(pool, logger)
	
	t.Run("without cache does nothing", func(t *testing.T) {
		err := repo.InvalidateCache(context.Background(), "test:*")
		assert.NoError(t, err)
	})
	
	t.Run("with cache calls clear", func(t *testing.T) {
		cache := new(mockCache)
		repo.WithCache(cache)
		
		ctx := context.Background()
		pattern := "users:*"
		
		cache.On("Clear", ctx, pattern).Return(nil)
		
		err := repo.InvalidateCache(ctx, pattern)
		assert.NoError(t, err)
		
		cache.AssertExpectations(t)
	})
}